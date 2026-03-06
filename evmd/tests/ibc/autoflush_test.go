package ibc

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/testutil"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Test constants
const (
	AutoFlushInitialTokenAmount int64 = 1_000_000_000_000_000_000 // 1 token with 18 decimals
	AutoFlushDelegationAmount   int64 = 100_000_000_000_000_000   // 0.1 token for delegation
	AutoFlushTransferAmount     int64 = 50_000_000_000_000_000    // 0.05 token for transfers
	AutoFlushNativeAmount       int64 = 1_000_000_000_000_000_000 // 1 native token (larger to avoid fractional issues)
	AutoFlushSenderIndex              = 1
)

// Test suite for auto-flush behavior with various operation combinations
type AutoFlushTestSuite struct {
	suite.Suite

	coordinator *evmibctesting.Coordinator
	chain       *evmibctesting.TestChain
}

func (suite *AutoFlushTestSuite) SetupTest() {
	suite.coordinator = evmibctesting.NewCoordinator(suite.T(), 1, 0, integration.SetupEvmd)
	suite.chain = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
}

func TestAutoFlushTestSuite(t *testing.T) {
	suite.Run(t, new(AutoFlushTestSuite))
}

// Helper: Deploy and setup ERC20 token
func (suite *AutoFlushTestSuite) deployAndRegisterERC20(name, symbol string) (common.Address, evmtypes.CompiledContract) {
	evmApp := suite.chain.App.(*evmd.EVMD)

	// Deploy ERC20
	erc20ContractData := contracts.ERC20MinterBurnerDecimalsContract
	deploymentData := testutiltypes.ContractDeploymentData{
		Contract:        erc20ContractData,
		ConstructorArgs: []interface{}{name, symbol, uint8(18)},
	}

	erc20Addr, err := DeployContract(suite.T(), suite.chain, deploymentData)
	suite.chain.NextBlock()
	suite.Require().NoError(err)

	// Register ERC20
	_, err = evmApp.Erc20Keeper.RegisterERC20(suite.chain.GetContext(), &erc20types.MsgRegisterERC20{
		Signer:         evmApp.AccountKeeper.GetModuleAddress("gov").String(),
		Erc20Addresses: []string{erc20Addr.Hex()},
	})
	suite.Require().NoError(err)
	suite.chain.NextBlock()

	return erc20Addr, erc20ContractData
}

// Helper: Get validator address
func (suite *AutoFlushTestSuite) getValidatorAddress() string {
	evmApp := suite.chain.App.(*evmd.EVMD)
	ctx := suite.chain.GetContext()

	vals, err := evmApp.StakingKeeper.GetAllValidators(ctx)
	suite.Require().NoError(err)
	suite.Require().Greater(len(vals), 0, "no validators found")

	return vals[0].OperatorAddress
}

// Helper: Mint tokens to address
func (suite *AutoFlushTestSuite) mintTokens(tokenAddr common.Address, erc20Data evmtypes.CompiledContract, recipient common.Address, amount *big.Int) {
	evmApp := suite.chain.App.(*evmd.EVMD)
	deployerAddr := common.BytesToAddress(suite.chain.SenderPrivKey.PubKey().Address().Bytes())

	stateDB := testutil.NewStateDB(suite.chain.GetContext(), evmApp.EVMKeeper)
	_, err := evmApp.GetEVMKeeper().CallEVM(
		suite.chain.GetContext(),
		stateDB,
		erc20Data.ABI,
		deployerAddr,
		tokenAddr,
		true,
		false,
		nil,
		"mint",
		recipient,
		amount,
	)
	suite.Require().NoError(err)
	suite.chain.NextBlock()
}

// Helper: Fund contract with native tokens
func (suite *AutoFlushTestSuite) fundContractNative(contractAddr sdk.AccAddress, amount sdkmath.Int) {
	evmApp := suite.chain.App.(*evmd.EVMD)
	bondDenom, err := evmApp.StakingKeeper.BondDenom(suite.chain.GetContext())
	suite.Require().NoError(err)

	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, amount))
	err = evmApp.GetBankKeeper().MintCoins(suite.chain.GetContext(), "mint", coins)
	suite.Require().NoError(err)

	err = evmApp.GetBankKeeper().SendCoinsFromModuleToAccount(
		suite.chain.GetContext(),
		"mint",
		contractAddr,
		coins,
	)
	suite.Require().NoError(err)
	suite.chain.NextBlock()
}

// Helper: Count events and print them
func (suite *AutoFlushTestSuite) countEvents(ctx sdk.Context) int {
	return len(ctx.EventManager().Events())
}

// Helper: Check balances and events
func (suite *AutoFlushTestSuite) verifyState(
	label string,
	expectedEventCount int,
	expectedBalances map[common.Address]map[common.Address]*big.Int, // tokenAddr -> holderAddr -> balance
	expectedNativeBalances map[string]sdkmath.Int, // bech32 address -> balance
	expectedAccountsCreated int,
) {
	evmApp := suite.chain.App.(*evmd.EVMD)
	ctx := suite.chain.GetContext()

	// Check event count
	actualEventCount := suite.countEvents(ctx)
	suite.Require().Equal(expectedEventCount, actualEventCount,
		"%s: event count mismatch - expected %d, got %d", label, expectedEventCount, actualEventCount)

	// Check ERC20 balances
	for tokenAddr, holders := range expectedBalances {
		tokenPairID := evmApp.Erc20Keeper.GetTokenPairID(ctx, "erc20:"+tokenAddr.Hex())
		tokenPair, found := evmApp.Erc20Keeper.GetTokenPair(ctx, tokenPairID)
		suite.Require().True(found, "%s: token pair not found for %s", label, tokenAddr.Hex())

		erc20Data := contracts.ERC20MinterBurnerDecimalsContract
		for holderAddr, expectedBal := range holders {
			actualBal := evmApp.GetErc20Keeper().BalanceOf(ctx, erc20Data.ABI, tokenPair.GetERC20Contract(), holderAddr)
			suite.Require().Equal(expectedBal.String(), actualBal.String(),
				"%s: ERC20 balance mismatch for %s holding %s", label, holderAddr.Hex(), tokenAddr.Hex())
		}
	}

	// Check native balances
	for addrStr, expectedBal := range expectedNativeBalances {
		bondDenom, err := evmApp.StakingKeeper.BondDenom(ctx)
		suite.Require().NoError(err)

		addr, err := sdk.AccAddressFromBech32(addrStr)
		suite.Require().NoError(err)

		actualBal := evmApp.GetBankKeeper().GetBalance(ctx, addr, bondDenom)
		suite.Require().Equal(expectedBal.String(), actualBal.Amount.String(),
			"%s: native balance mismatch for %s", label, addrStr)
	}

	// TODO: Check account creation count if needed
	_ = expectedAccountsCreated
}

// Scenario 1: Transfer ERC20 -> Delegate -> Transfer ERC20
func (suite *AutoFlushTestSuite) TestScenario1_TransferDelegateTransfer() {
	suite.SetupTest()

	evmApp := suite.chain.App.(*evmd.EVMD)
	ctx := suite.chain.GetContext()

	// Deploy test contract
	contractData, err := contracts.LoadSequentialOperationsTester()
	suite.Require().NoError(err)

	deploymentData := testutiltypes.ContractDeploymentData{
		Contract:        contractData,
		ConstructorArgs: []interface{}{},
	}

	contractAddr, err := DeployContract(suite.T(), suite.chain, deploymentData)
	suite.chain.NextBlock()
	suite.Require().NoError(err)

	contractAddrSDK := sdk.AccAddress(contractAddr.Bytes())

	// Deploy and register ERC20
	tokenAddr, erc20Data := suite.deployAndRegisterERC20("TestToken", "TT")

	// Mint tokens to contract
	suite.mintTokens(tokenAddr, erc20Data, contractAddr, big.NewInt(AutoFlushInitialTokenAmount))

	// Fund contract with native tokens for delegation
	suite.fundContractNative(contractAddrSDK, sdkmath.NewInt(AutoFlushDelegationAmount*2))

	// Get recipient and validator
	senderAccount := suite.chain.SenderAccounts[AutoFlushSenderIndex]
	recipientAddr := common.BytesToAddress(senderAccount.SenderAccount.GetAddress().Bytes())
	validatorAddr := suite.getValidatorAddress()

	// Get balances before
	ctx = suite.chain.GetContext()
	contractBalBefore := evmApp.GetErc20Keeper().BalanceOf(ctx, erc20Data.ABI, tokenAddr, contractAddr)
	recipientBalBefore := evmApp.GetErc20Keeper().BalanceOf(ctx, erc20Data.ABI, tokenAddr, recipientAddr)

	// Pack the call data
	callData, err := contractData.ABI.Pack(
		"scenario1_transferDelegateTransfer",
		tokenAddr,
		recipientAddr,
		big.NewInt(AutoFlushTransferAmount),
		validatorAddr,
		big.NewInt(AutoFlushDelegationAmount),
	)
	suite.Require().NoError(err)

	// Execute via SendEvmTx to get proper event tracking
	res, _, _, err := suite.chain.SendEvmTx(
		senderAccount,
		AutoFlushSenderIndex,
		contractAddr,
		big.NewInt(0),
		callData,
		0,
	)
	suite.Require().NoError(err)

	// Get balances after
	ctx = suite.chain.GetContext()
	contractBalAfter := evmApp.GetErc20Keeper().BalanceOf(ctx, erc20Data.ABI, tokenAddr, contractAddr)
	recipientBalAfter := evmApp.GetErc20Keeper().BalanceOf(ctx, erc20Data.ABI, tokenAddr, recipientAddr)

	expectedDelta := new(big.Int).Mul(big.NewInt(AutoFlushTransferAmount), big.NewInt(2))
	actualDelta := new(big.Int).Sub(contractBalBefore, contractBalAfter)

	// Verify balances
	suite.Require().Equal(expectedDelta.String(), actualDelta.String(), "transfer amount mismatch")
	suite.Require().Equal(
		new(big.Int).Mul(big.NewInt(AutoFlushTransferAmount), big.NewInt(2)).String(),
		new(big.Int).Sub(recipientBalAfter, recipientBalBefore).String(),
		"recipient should receive 2x transfer amount",
	)

	// Verify delegation occurred
	delegations, err := evmApp.StakingKeeper.GetAllDelegatorDelegations(ctx, contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(len(delegations), 1, "should have at 1 delegation")
	bondedTokens, err := evmApp.StakingKeeper.GetDelegatorBonded(suite.chain.GetContext(), contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(AutoFlushDelegationAmount, bondedTokens.Int64())

	suite.Require().Equal(DelegationEventCount+EVMEventCount, len(res.Events)) // 8 events, no gas token transfer
}

// Scenario 2: Transfer ERC20 -> Delegate (reverted & caught) -> Transfer ERC20
func (suite *AutoFlushTestSuite) TestScenario2_TransferDelegateRevertTransfer() {
	suite.SetupTest()

	evmApp := suite.chain.App.(*evmd.EVMD)
	ctx := suite.chain.GetContext()

	// Deploy test contract
	contractData, err := contracts.LoadSequentialOperationsTester()
	suite.Require().NoError(err)

	deploymentData := testutiltypes.ContractDeploymentData{
		Contract:        contractData,
		ConstructorArgs: []interface{}{},
	}

	contractAddr, err := DeployContract(suite.T(), suite.chain, deploymentData)
	suite.chain.NextBlock()
	suite.Require().NoError(err)

	// Deploy and register ERC20
	tokenAddr, erc20Data := suite.deployAndRegisterERC20("TestToken2", "TT2")

	// Mint tokens to contract
	suite.mintTokens(tokenAddr, erc20Data, contractAddr, big.NewInt(AutoFlushInitialTokenAmount))

	// Get recipient and validator
	senderAccount := suite.chain.SenderAccounts[AutoFlushSenderIndex]
	recipientAddr := common.BytesToAddress(senderAccount.SenderAccount.GetAddress().Bytes())
	validatorAddr := suite.getValidatorAddress()

	// Get balances before
	ctx = suite.chain.GetContext()
	contractBalBefore := evmApp.GetErc20Keeper().BalanceOf(ctx, erc20Data.ABI, tokenAddr, contractAddr)
	recipientBalBefore := evmApp.GetErc20Keeper().BalanceOf(ctx, erc20Data.ABI, tokenAddr, recipientAddr)

	// Pack the call data with excessive delegation amount (will revert and be caught)
	excessiveAmount := new(big.Int).Mul(big.NewInt(AutoFlushDelegationAmount), big.NewInt(1000))
	callData, err := contractData.ABI.Pack(
		"scenario2_transferDelegateRevertTransfer",
		tokenAddr,
		recipientAddr,
		big.NewInt(AutoFlushTransferAmount),
		validatorAddr,
		excessiveAmount, // Will fail - insufficient balance
	)
	suite.Require().NoError(err)

	// Execute via SendEvmTx to get proper event tracking
	res, _, _, err := suite.chain.SendEvmTx(
		senderAccount,
		AutoFlushSenderIndex,
		contractAddr,
		big.NewInt(0),
		callData,
		0,
	)
	suite.Require().NoError(err)

	// Get balances after
	ctx = suite.chain.GetContext()
	contractBalAfter := evmApp.GetErc20Keeper().BalanceOf(ctx, erc20Data.ABI, tokenAddr, contractAddr)
	recipientBalAfter := evmApp.GetErc20Keeper().BalanceOf(ctx, erc20Data.ABI, tokenAddr, recipientAddr)

	expectedDelta := new(big.Int).Mul(big.NewInt(AutoFlushTransferAmount), big.NewInt(2))
	actualDelta := new(big.Int).Sub(contractBalBefore, contractBalAfter)

	// Verify balances - both transfers should succeed even though delegate failed
	suite.Require().Equal(expectedDelta.String(), actualDelta.String(), "transfer amount mismatch")
	suite.Require().Equal(
		new(big.Int).Mul(big.NewInt(AutoFlushTransferAmount), big.NewInt(2)).String(),
		new(big.Int).Sub(recipientBalAfter, recipientBalBefore).String(),
		"recipient should receive 2x transfer amount",
	)

	// Verify NO delegation occurred (it was reverted)
	contractAddrSDK := sdk.AccAddress(contractAddr.Bytes())
	delegations, err := evmApp.StakingKeeper.GetAllDelegatorDelegations(ctx, contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(0, len(delegations), "should have no delegations since it reverted")
	bondedTokens, err := evmApp.StakingKeeper.GetDelegatorBonded(suite.chain.GetContext(), contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(int64(0), bondedTokens.Int64())

	suite.Require().Equal(EVMEventCount, len(res.Events))
}

// Scenario 3: Native transfer -> Delegate -> Native transfer
func (suite *AutoFlushTestSuite) TestScenario3_NativeTransferDelegateNativeTransfer() {
	suite.SetupTest()

	evmApp := suite.chain.App.(*evmd.EVMD)
	ctx := suite.chain.GetContext()

	// Deploy test contract
	contractData, err := contracts.LoadSequentialOperationsTester()
	suite.Require().NoError(err)

	deploymentData := testutiltypes.ContractDeploymentData{
		Contract:        contractData,
		ConstructorArgs: []interface{}{},
	}

	contractAddr, err := DeployContract(suite.T(), suite.chain, deploymentData)
	suite.chain.NextBlock()
	suite.Require().NoError(err)

	contractAddrSDK := sdk.AccAddress(contractAddr.Bytes())
	senderAccount := suite.chain.SenderAccounts[AutoFlushSenderIndex]

	// Fund contract via bank module for all operations (both native transfers and delegation)
	// Convert from wei (18 decimals) to aatom (6 decimals) by dividing by 1e12
	totalAmountWei := AutoFlushNativeAmount*2 + AutoFlushDelegationAmount
	totalAmountAatom := sdkmath.NewInt(totalAmountWei)
	suite.fundContractNative(contractAddrSDK, totalAmountAatom)

	// Get recipient and validator
	recipientAddr := common.BytesToAddress(senderAccount.SenderAccount.GetAddress().Bytes())
	recipientAddrSDK := senderAccount.SenderAccount.GetAddress()
	validatorAddr := suite.getValidatorAddress()

	// Get balances before
	ctx = suite.chain.GetContext()
	bondDenom, err := evmApp.StakingKeeper.BondDenom(ctx)
	suite.Require().NoError(err)

	contractBalBefore := evmApp.GetBankKeeper().GetBalance(ctx, contractAddrSDK, bondDenom)
	recipientBalBefore := evmApp.GetBankKeeper().GetBalance(ctx, recipientAddrSDK, bondDenom)

	// Pack the call data
	callData, err := contractData.ABI.Pack(
		"scenario3_nativeTransferDelegateNativeTransfer",
		recipientAddr,
		big.NewInt(AutoFlushNativeAmount),
		validatorAddr,
		big.NewInt(AutoFlushDelegationAmount),
	)
	suite.Require().NoError(err)

	// Execute via SendEvmTx to get proper event tracking
	res, _, _, err := suite.chain.SendEvmTx(
		senderAccount,
		AutoFlushSenderIndex,
		contractAddr,
		big.NewInt(0),
		callData,
		0,
	)
	suite.Require().NoError(err)

	// Get balances after
	ctx = suite.chain.GetContext()
	contractBalAfter := evmApp.GetBankKeeper().GetBalance(ctx, contractAddrSDK, bondDenom)
	recipientBalAfter := evmApp.GetBankKeeper().GetBalance(ctx, recipientAddrSDK, bondDenom)

	// Verify balances - contract should lose 2x native transfer + delegation (in bank units)
	expectedContractDeltaWei := sdkmath.NewInt(AutoFlushNativeAmount*2 + AutoFlushDelegationAmount)
	actualContractDelta := contractBalBefore.Amount.Sub(contractBalAfter.Amount)

	// Verify recipient received 2x native amount (in bank units)
	expectedRecipientDeltaWei := sdkmath.NewInt(AutoFlushNativeAmount * 2)
	actualRecipientDelta := recipientBalAfter.Amount.Sub(recipientBalBefore.Amount)

	suite.Require().Equal(expectedContractDeltaWei.String(), actualContractDelta.String(), "contract balance delta mismatch")
	suite.Require().Equal(expectedRecipientDeltaWei.String(), actualRecipientDelta.String(), "recipient should receive 2x native amount")

	// Verify delegation occurred
	delegations, err := evmApp.StakingKeeper.GetAllDelegatorDelegations(ctx, contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(len(delegations), 1, "should have 1 delegation")
	bondedTokens, err := evmApp.StakingKeeper.GetDelegatorBonded(suite.chain.GetContext(), contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(AutoFlushDelegationAmount, bondedTokens.Int64())

	suite.Require().Equal(DelegationEventCount+EVMEventCount, len(res.Events))
}

// Scenario 4: Native transfer -> Delegate (reverted & caught) -> Native transfer
func (suite *AutoFlushTestSuite) TestScenario4_NativeTransferDelegateRevertNativeTransfer() {
	suite.SetupTest()

	evmApp := suite.chain.App.(*evmd.EVMD)
	ctx := suite.chain.GetContext()

	// Deploy test contract
	contractData, err := contracts.LoadSequentialOperationsTester()
	suite.Require().NoError(err)

	deploymentData := testutiltypes.ContractDeploymentData{
		Contract:        contractData,
		ConstructorArgs: []interface{}{},
	}

	contractAddr, err := DeployContract(suite.T(), suite.chain, deploymentData)
	suite.chain.NextBlock()
	suite.Require().NoError(err)

	contractAddrSDK := sdk.AccAddress(contractAddr.Bytes())

	// Fund contract with native tokens for transfers via direct EVM transfer
	senderAccount := suite.chain.SenderAccounts[AutoFlushSenderIndex]
	nativeTransferValue := big.NewInt(AutoFlushNativeAmount * 2) // 2x transfers
	_, _, _, err = suite.chain.SendEvmTx(
		senderAccount,
		AutoFlushSenderIndex,
		contractAddr,
		nativeTransferValue,
		nil, // no data, just value transfer
		0,
	)
	suite.Require().NoError(err)
	suite.chain.NextBlock()

	// Get recipient and validator
	recipientAddr := common.BytesToAddress(senderAccount.SenderAccount.GetAddress().Bytes())
	recipientAddrSDK := senderAccount.SenderAccount.GetAddress()
	validatorAddr := suite.getValidatorAddress()

	// Get balances before
	bondDenom, err := evmApp.StakingKeeper.BondDenom(ctx)
	suite.Require().NoError(err)

	contractBalBefore := evmApp.GetBankKeeper().GetBalance(ctx, contractAddrSDK, bondDenom)
	recipientBalBefore := evmApp.GetBankKeeper().GetBalance(ctx, recipientAddrSDK, bondDenom)

	// Pack the call data with excessive delegation amount (will revert and be caught)
	excessiveAmount := new(big.Int).Mul(big.NewInt(AutoFlushDelegationAmount), big.NewInt(1000))
	callData, err := contractData.ABI.Pack(
		"scenario4_nativeTransferDelegateRevertNativeTransfer",
		recipientAddr,
		big.NewInt(AutoFlushNativeAmount),
		validatorAddr,
		excessiveAmount, // Will fail - insufficient balance
	)
	suite.Require().NoError(err)

	// Execute via SendEvmTx to get proper event tracking
	res, _, _, err := suite.chain.SendEvmTx(
		senderAccount,
		AutoFlushSenderIndex,
		contractAddr,
		big.NewInt(0),
		callData,
		0,
	)
	suite.Require().NoError(err)

	// Get balances after
	ctx = suite.chain.GetContext()
	contractBalAfter := evmApp.GetBankKeeper().GetBalance(ctx, contractAddrSDK, bondDenom)
	recipientBalAfter := evmApp.GetBankKeeper().GetBalance(ctx, recipientAddrSDK, bondDenom)

	// Bank balances are in Cosmos units (6 decimals), EVM amounts are in wei (18 decimals)

	// Verify balances - contract should lose only 2x native transfer (delegation reverted, in bank units)
	expectedContractDeltaWei := sdkmath.NewInt(AutoFlushNativeAmount * 2)
	actualContractDelta := contractBalBefore.Amount.Sub(contractBalAfter.Amount)

	// Verify recipient received 2x native amount (in bank units)
	expectedRecipientDeltaWei := sdkmath.NewInt(AutoFlushNativeAmount * 2)
	actualRecipientDelta := recipientBalAfter.Amount.Sub(recipientBalBefore.Amount)

	suite.Require().Equal(expectedContractDeltaWei.String(), actualContractDelta.String(), "contract balance delta mismatch")
	suite.Require().Equal(expectedRecipientDeltaWei.String(), actualRecipientDelta.String(), "recipient should receive 2x native amount")

	// Verify NO delegation occurred (it was reverted)
	delegations, err := evmApp.StakingKeeper.GetAllDelegatorDelegations(ctx, contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(0, len(delegations), "should have no delegations since it reverted")
	bondedTokens, err := evmApp.StakingKeeper.GetDelegatorBonded(suite.chain.GetContext(), contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(int64(0), bondedTokens.Int64())

	suite.Require().Equal(EVMEventCount, len(res.Events)) // no delegation in middle, so we bundle the two events transfer events into one
}

// Scenario 5: Delegate -> Create Contract -> Delegate
func (suite *AutoFlushTestSuite) TestScenario5_DelegateCreateDelegate() {
	suite.SetupTest()

	evmApp := suite.chain.App.(*evmd.EVMD)
	ctx := suite.chain.GetContext()

	// Deploy test contract
	contractData, err := contracts.LoadContractCreationTester()
	suite.Require().NoError(err)

	deploymentData := testutiltypes.ContractDeploymentData{
		Contract:        contractData,
		ConstructorArgs: []interface{}{},
	}

	contractAddr, err := DeployContract(suite.T(), suite.chain, deploymentData)
	suite.chain.NextBlock()
	suite.Require().NoError(err)

	contractAddrSDK := sdk.AccAddress(contractAddr.Bytes())

	// Fund contract for delegations via bank module
	delegationAmountAatom := sdkmath.NewInt(int64(AutoFlushDelegationAmount * 2))
	suite.fundContractNative(contractAddrSDK, delegationAmountAatom)

	// Get validator
	senderAccount := suite.chain.SenderAccounts[AutoFlushSenderIndex]
	validatorAddr := suite.getValidatorAddress()

	// Value to send when creating the contract
	creationValue := big.NewInt(AutoFlushNativeAmount)

	// Pack the call data
	callData, err := contractData.ABI.Pack(
		"scenario4_delegateCreateDelegate",
		validatorAddr,
		big.NewInt(AutoFlushDelegationAmount),
		big.NewInt(1).Quo(creationValue, big.NewInt(10)),
		big.NewInt(AutoFlushDelegationAmount),
	)
	suite.Require().NoError(err)

	// Execute via SendEvmTx with value for contract creation
	res, _, _, err := suite.chain.SendEvmTx(
		senderAccount,
		AutoFlushSenderIndex,
		contractAddr,
		creationValue, // Send value for the new contract creation
		callData,
		0,
	)
	suite.Require().NoError(err)

	// Verify 1 delegations occurred
	delegations, err := evmApp.StakingKeeper.GetAllDelegatorDelegations(ctx, contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(1, len(delegations), "should have 1 delegation")
	bondedTokens, err := evmApp.StakingKeeper.GetDelegatorBonded(suite.chain.GetContext(), contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(AutoFlushDelegationAmount*2, bondedTokens.Int64())

	suite.Require().Equal(DelegationEventCount+DelegationEventCount+WithdrawalNoTokensEventCount+EVMEventCount, len(res.Events))
}

// Scenario 6: Delegate -> Create Contract (reverted & caught) -> Delegate
func (suite *AutoFlushTestSuite) TestScenario6_DelegateCreateRevertDelegate() {
	suite.SetupTest()

	evmApp := suite.chain.App.(*evmd.EVMD)
	ctx := suite.chain.GetContext()

	// Deploy test contract
	contractData, err := contracts.LoadContractCreationTester()
	suite.Require().NoError(err)

	deploymentData := testutiltypes.ContractDeploymentData{
		Contract:        contractData,
		ConstructorArgs: []interface{}{},
	}

	contractAddr, err := DeployContract(suite.T(), suite.chain, deploymentData)
	suite.chain.NextBlock()
	suite.Require().NoError(err)

	contractAddrSDK := sdk.AccAddress(contractAddr.Bytes())

	// Fund contract for delegations via bank module
	delegationAmountAatom := sdkmath.NewInt(int64(AutoFlushDelegationAmount * 2))
	suite.fundContractNative(contractAddrSDK, delegationAmountAatom)

	// Get validator
	senderAccount := suite.chain.SenderAccounts[AutoFlushSenderIndex]
	validatorAddr := suite.getValidatorAddress()

	// Excessive value that will cause contract creation to fail
	excessiveValue := new(big.Int).Mul(big.NewInt(AutoFlushNativeAmount), big.NewInt(1000))

	// Pack the call data
	callData, err := contractData.ABI.Pack(
		"scenario5_delegateCreateRevertDelegate",
		validatorAddr,
		big.NewInt(AutoFlushDelegationAmount),
		excessiveValue, // Will fail - insufficient value
		big.NewInt(AutoFlushDelegationAmount),
	)
	suite.Require().NoError(err)

	// Execute via SendEvmTx
	res, _, _, err := suite.chain.SendEvmTx(
		senderAccount,
		AutoFlushSenderIndex,
		contractAddr,
		big.NewInt(0),
		callData,
		0,
	)
	suite.Require().NoError(err)

	// Verify 1 delegation occurred
	delegations, err := evmApp.StakingKeeper.GetAllDelegatorDelegations(ctx, contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(1, len(delegations), "should have 1 delegation")
	bondedTokens, err := evmApp.StakingKeeper.GetDelegatorBonded(suite.chain.GetContext(), contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(AutoFlushDelegationAmount*2, bondedTokens.Int64())

	suite.Require().Equal(DelegationEventCount+DelegationEventCount+WithdrawalNoTokensEventCount+EVMEventCount, len(res.Events))
}

// Scenario 7: Create+Revert (caught) -> Delegate -> Create+Revert (caught)
func (suite *AutoFlushTestSuite) TestScenario7_CreateRevertDelegateCreateRevert() {
	suite.SetupTest()

	evmApp := suite.chain.App.(*evmd.EVMD)
	ctx := suite.chain.GetContext()

	// Deploy test contract
	contractData, err := contracts.LoadContractCreationTester()
	suite.Require().NoError(err)

	deploymentData := testutiltypes.ContractDeploymentData{
		Contract:        contractData,
		ConstructorArgs: []interface{}{},
	}

	contractAddr, err := DeployContract(suite.T(), suite.chain, deploymentData)
	suite.chain.NextBlock()
	suite.Require().NoError(err)

	contractAddrSDK := sdk.AccAddress(contractAddr.Bytes())
	senderAccount := suite.chain.SenderAccounts[AutoFlushSenderIndex]

	// Fund contract for delegation via bank module
	delegationAmountAatom := sdkmath.NewInt(int64(AutoFlushDelegationAmount))
	suite.fundContractNative(contractAddrSDK, delegationAmountAatom)

	// Fund contract with some EVM balance for contract creations (even though they revert)
	// This is needed because the contract needs balance to create sub-contracts
	fundingAmount := big.NewInt(AutoFlushNativeAmount)
	_, _, _, err = suite.chain.SendEvmTx(
		senderAccount,
		AutoFlushSenderIndex,
		contractAddr,
		fundingAmount,
		nil,
		0,
	)
	suite.Require().NoError(err)
	suite.chain.NextBlock()

	// Get validator
	validatorAddr := suite.getValidatorAddress()

	// Values for contract creations (use 0 to avoid balance issues with reverts)
	creationValue1 := big.NewInt(0)
	creationValue2 := big.NewInt(0)

	// Pack the call data
	callData, err := contractData.ABI.Pack(
		"scenario6_createRevertDelegateCreateRevert",
		creationValue1,
		validatorAddr,
		big.NewInt(AutoFlushDelegationAmount),
		creationValue2,
	)
	suite.Require().NoError(err)

	// Execute via SendEvmTx (no value needed since contract creations use 0 and revert)
	res, _, _, err := suite.chain.SendEvmTx(
		senderAccount,
		AutoFlushSenderIndex,
		contractAddr,
		big.NewInt(0),
		callData,
		0,
	)
	suite.Require().NoError(err)

	// Verify 1 delegation occurred
	delegations, err := evmApp.StakingKeeper.GetAllDelegatorDelegations(ctx, contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(1, len(delegations), "should have 1 delegation")
	bondedTokens, err := evmApp.StakingKeeper.GetDelegatorBonded(suite.chain.GetContext(), contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(AutoFlushDelegationAmount, bondedTokens.Int64())

	suite.Require().Equal(DelegationEventCount+EVMEventCount, len(res.Events))
}

// Scenario 8: Create+Send -> Delegate (reverted & caught) -> Send more
func (suite *AutoFlushTestSuite) TestScenario8_CreateDelegateRevertSend() {
	suite.SetupTest()

	evmApp := suite.chain.App.(*evmd.EVMD)
	ctx := suite.chain.GetContext()

	// Deploy test contract
	contractData, err := contracts.LoadContractCreationTester()
	suite.Require().NoError(err)

	deploymentData := testutiltypes.ContractDeploymentData{
		Contract:        contractData,
		ConstructorArgs: []interface{}{},
	}

	contractAddr, err := DeployContract(suite.T(), suite.chain, deploymentData)
	suite.chain.NextBlock()
	suite.Require().NoError(err)

	// Get validator
	senderAccount := suite.chain.SenderAccounts[AutoFlushSenderIndex]
	validatorAddr := suite.getValidatorAddress()

	// Values for operations
	creationValue := big.NewInt(AutoFlushNativeAmount)
	sendAmount := big.NewInt(AutoFlushNativeAmount / 2)
	excessiveDelegateAmount := new(big.Int).Mul(big.NewInt(AutoFlushDelegationAmount), big.NewInt(1000))

	// Pack the call data
	callData, err := contractData.ABI.Pack(
		"scenario7_createDelegateRevertSend",
		creationValue,
		validatorAddr,
		excessiveDelegateAmount, // Will fail - insufficient balance
		sendAmount,
	)
	suite.Require().NoError(err)

	// Execute via SendEvmTx with value for contract creation and sends
	totalValue := new(big.Int).Add(creationValue, sendAmount)
	res, _, _, err := suite.chain.SendEvmTx(
		senderAccount,
		AutoFlushSenderIndex,
		contractAddr,
		totalValue,
		callData,
		0,
	)
	suite.Require().NoError(err)

	// Verify NO delegation occurred (it was reverted)
	contractAddrSDK := sdk.AccAddress(contractAddr.Bytes())
	delegations, err := evmApp.StakingKeeper.GetAllDelegatorDelegations(ctx, contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(0, len(delegations), "should have no delegations since it reverted")

	suite.Require().Equal(EVMEventCount, len(res.Events))
}

// Scenario 9: Create+Revert (caught) -> Delegate -> Create+Success
func (suite *AutoFlushTestSuite) TestScenario9_CreateRevertDelegateCreateSuccess() {
	suite.SetupTest()

	evmApp := suite.chain.App.(*evmd.EVMD)

	// Deploy test contract
	contractData, err := contracts.LoadContractCreationTester()
	suite.Require().NoError(err)

	deploymentData := testutiltypes.ContractDeploymentData{
		Contract:        contractData,
		ConstructorArgs: []interface{}{},
	}

	contractAddr, err := DeployContract(suite.T(), suite.chain, deploymentData)
	suite.chain.NextBlock()
	suite.Require().NoError(err)

	contractAddrSDK := sdk.AccAddress(contractAddr.Bytes())
	senderAccount := suite.chain.SenderAccounts[AutoFlushSenderIndex]

	// Fund contract for delegation via bank module
	delegationAmountAatom := sdkmath.NewInt(int64(AutoFlushDelegationAmount))
	suite.fundContractNative(contractAddrSDK, delegationAmountAatom)

	// Fund contract with EVM balance for contract creations
	// Need enough for the successful creation (reverted one returns the value)
	fundingAmount := big.NewInt(AutoFlushNativeAmount * 2) // Extra buffer
	_, _, _, err = suite.chain.SendEvmTx(
		senderAccount,
		AutoFlushSenderIndex,
		contractAddr,
		fundingAmount,
		nil,
		0,
	)
	suite.Require().NoError(err)
	suite.chain.NextBlock()

	// Get validator
	validatorAddr := suite.getValidatorAddress()

	// Values for operations
	revertCreationValue := big.NewInt(AutoFlushNativeAmount / 2)
	successCreationValue := big.NewInt(AutoFlushNativeAmount / 2)

	// Get balances before
	ctx := suite.chain.GetContext()
	bondDenom, err := evmApp.StakingKeeper.BondDenom(ctx)
	suite.Require().NoError(err)
	contractBalBefore := evmApp.GetBankKeeper().GetBalance(ctx, contractAddrSDK, bondDenom)

	// Pack the call data
	callData, err := contractData.ABI.Pack(
		"scenario8_createRevertDelegateCreateSuccess",
		revertCreationValue,
		validatorAddr,
		big.NewInt(AutoFlushDelegationAmount),
		successCreationValue,
	)
	suite.Require().NoError(err)

	// Execute via SendEvmTx
	res, _, _, err := suite.chain.SendEvmTx(
		senderAccount,
		AutoFlushSenderIndex,
		contractAddr,
		big.NewInt(0),
		callData,
		0,
	)
	suite.Require().NoError(err)

	// Verify final state
	ctx = suite.chain.GetContext()

	// Check delegation occurred
	delegations, err := evmApp.StakingKeeper.GetAllDelegatorDelegations(ctx, contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(1, len(delegations), "should have 1 delegation")
	bondedTokens, err := evmApp.StakingKeeper.GetDelegatorBonded(suite.chain.GetContext(), contractAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(AutoFlushDelegationAmount, bondedTokens.Int64())

	// Check created contracts count
	stateDB := testutil.NewStateDB(ctx, evmApp.EVMKeeper)

	countResult, err := evmApp.GetEVMKeeper().CallEVM(
		ctx,
		stateDB,
		contractData.ABI,
		contractAddr,
		contractAddr,
		true,
		false,
		nil,
		"getCreatedContractsCount",
	)
	suite.Require().NoError(err)

	var createdCount *big.Int
	err = contractData.ABI.UnpackIntoInterface(&createdCount, "getCreatedContractsCount", countResult.Ret)
	suite.Require().NoError(err)
	suite.Require().Equal(int64(1), createdCount.Int64(), "should have 1 created contract (reverted one doesn't count)")

	// Get the created contract address
	addrResult, err := evmApp.GetEVMKeeper().CallEVM(
		ctx,
		stateDB,
		contractData.ABI,
		contractAddr,
		contractAddr,
		true,
		false,
		nil,
		"getCreatedContract",
		big.NewInt(0),
	)
	suite.Require().NoError(err)

	var createdContractAddr common.Address
	err = contractData.ABI.UnpackIntoInterface(&createdContractAddr, "getCreatedContract", addrResult.Ret)
	suite.Require().NoError(err)

	// Check the created contract's balance
	createdContractBalance := stateDB.GetBalance(createdContractAddr)
	suite.Require().Equal(successCreationValue.String(), createdContractBalance.String(),
		"created contract should have the creation value")

	// Check main contract's bank balance decreased by delegation + created contract value
	contractBalAfter := evmApp.GetBankKeeper().GetBalance(ctx, contractAddrSDK, bondDenom)
	// Delta = delegation amount + successful creation value (reverted creation returns the value)
	expectedDelta := sdkmath.NewInt((AutoFlushDelegationAmount + successCreationValue.Int64()))
	actualDelta := contractBalBefore.Amount.Sub(contractBalAfter.Amount)
	suite.Require().Equal(expectedDelta.String(), actualDelta.String(), "bank balance should decrease by delegation + creation value")

	suite.Require().Equal(DelegationEventCount+EVMEventCount, len(res.Events))
}
