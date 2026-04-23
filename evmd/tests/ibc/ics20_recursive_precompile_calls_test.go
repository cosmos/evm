// Copied from https://github.com/cosmos/ibc-go/blob/7325bd2b00fd5e33d895770ec31b5be2f497d37a/modules/apps/transfer/transfer_test.go
// Why was this copied?
// This test suite was imported to validate that ExampleChain (an EVM-based chain)
// correctly supports IBC v1 token transfers using ibc-go’s Transfer module logic.
// The test ensures that ics20 precompile transfer (A → B) behave as expected across channels.
package ibc

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/precompiles/ics20"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	"github.com/cosmos/evm/utils"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	transfertypes "github.com/cosmos/ibc-go/v11/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v11/modules/core/02-client/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// Test constants
const (
	// Token amounts
	InitialTokenAmount int64 = 1_000_000_000_000_000_000 // 1 token with 18 decimals
	DelegationAmount   int64 = 1_000_000_000_000_000_000 // 1 token for delegation
	RewardAmount             = 100                       // 100 base units for rewards
	ExpectedRewards          = "50.000000000000000000"   // Expected reward amount after allocation

	// Test configuration
	SenderIndex   = 1
	TimeoutHeight = 110
)

// Test suite for ICS20 recursive precompile calls
// Tests the native balance handler bug where reverted distribution calls
// leave persistent bank events that are incorrectly aggregated

type ICS20RecursivePrecompileCallsTestSuite struct {
	suite.Suite

	coordinator *evmibctesting.Coordinator

	// testing chains used for convenience and readability
	chainA           *evmibctesting.TestChain
	chainAPrecompile *ics20.Precompile
	chainB           *evmibctesting.TestChain
	chainBPrecompile *ics20.Precompile
}

type stakingRewards struct {
	Delegator sdk.AccAddress
	Validator stakingtypes.Validator
	RewardAmt sdkmath.Int
}

func (suite *ICS20RecursivePrecompileCallsTestSuite) prepareStakingRewards(ctx sdk.Context, stkRs ...stakingRewards) (sdk.Context, error) {
	for _, r := range stkRs {
		// set distribution module account balance which pays out the rewards
		bondDenom, err := suite.chainA.App.(*evmd.EVMD).StakingKeeper.BondDenom(suite.chainA.GetContext())
		suite.Require().NoError(err)
		coins := sdk.NewCoins(sdk.NewCoin(bondDenom, r.RewardAmt))
		if err := suite.mintCoinsForDistrMod(ctx, coins); err != nil {
			return ctx, err
		}

		// allocate rewards to validator
		allocatedRewards := sdk.NewDecCoins(sdk.NewDecCoin(bondDenom, r.RewardAmt))
		if err := suite.chainA.App.(*evmd.EVMD).GetDistrKeeper().AllocateTokensToValidator(ctx, r.Validator, allocatedRewards); err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

func (suite *ICS20RecursivePrecompileCallsTestSuite) mintCoinsForDistrMod(ctx sdk.Context, amount sdk.Coins) error {
	// Mint tokens for the distribution module to simulate fee accrued
	if err := suite.chainA.App.(*evmd.EVMD).GetBankKeeper().MintCoins(
		ctx,
		minttypes.ModuleName,
		amount,
	); err != nil {
		return err
	}

	return suite.chainA.App.(*evmd.EVMD).GetBankKeeper().SendCoinsFromModuleToModule(
		ctx,
		minttypes.ModuleName,
		distrtypes.ModuleName,
		amount,
	)
}

// setupRevertingContractForTesting configures the contract for delegation and reward testing
func (suite *ICS20RecursivePrecompileCallsTestSuite) setupContractForTesting(
	contractAddr common.Address,
	contractData evmtypes.CompiledContract,
	senderAcc evmibctesting.SenderAccount,
) {
	evmAppA := suite.chainA.App.(*evmd.EVMD)
	senderAddr := senderAcc.SenderAccount.GetAddress()
	senderEVMAddr := common.BytesToAddress(senderAddr.Bytes())
	deployerAddr := common.BytesToAddress(suite.chainA.SenderPrivKey.PubKey().Address().Bytes())

	// Register ERC20 contract
	_, err := evmAppA.Erc20Keeper.RegisterERC20(suite.chainA.GetContext(), &erc20types.MsgRegisterERC20{
		Signer:         evmAppA.AccountKeeper.GetModuleAddress("gov").String(),
		Erc20Addresses: []string{contractAddr.Hex()},
	})
	suite.Require().NoError(err, "registering ERC20 token should succeed")
	suite.chainA.NextBlock()

	// Send native tokens to contract for delegation
	bondDenom, err := evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
	suite.Require().NoError(err)

	contractAddrBech32, err := sdk.AccAddressFromHexUnsafe(contractAddr.Hex()[2:])
	suite.Require().NoError(err)

	deployerAddrBech32 := sdk.AccAddress(deployerAddr.Bytes())
	deployerBalance := evmAppA.BankKeeper.GetBalance(suite.chainA.GetContext(), deployerAddrBech32, bondDenom)

	// Send delegation amount to contract
	sendAmount := sdkmath.NewInt(DelegationAmount)
	if deployerBalance.Amount.LT(sendAmount) {
		sendAmount = deployerBalance.Amount.Quo(sdkmath.NewInt(2))
	}

	err = evmAppA.BankKeeper.SendCoins(
		suite.chainA.GetContext(),
		deployerAddrBech32,
		contractAddrBech32,
		sdk.NewCoins(sdk.NewCoin(bondDenom, sendAmount)),
	)
	suite.Require().NoError(err, "sending native tokens to contract should succeed")

	// Mint ERC20 tokens
	stateDB := statedb.New(suite.chainA.GetContext(), evmAppA.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	_, err = evmAppA.GetEVMKeeper().CallEVM(
		suite.chainA.GetContext(),
		stateDB,
		contractData.ABI,
		deployerAddr,
		contractAddr,
		true,
		false,
		nil,
		"mint",
		senderEVMAddr,
		big.NewInt(InitialTokenAmount),
	)
	suite.Require().NoError(err, "mint call failed")
	suite.chainA.NextBlock()

	// Delegate tokens
	vals, err := evmAppA.StakingKeeper.GetAllValidators(suite.chainA.GetContext())
	suite.Require().NoError(err)

	stateDB = statedb.New(suite.chainA.GetContext(), evmAppA.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	_, err = evmAppA.GetEVMKeeper().CallEVM(
		suite.chainA.GetContext(),
		stateDB,
		contractData.ABI,
		deployerAddr,
		contractAddr,
		true,
		false,
		nil,
		"delegate",
		vals[0].OperatorAddress,
		big.NewInt(DelegationAmount),
	)
	suite.Require().NoError(err)

	// Verify delegation
	valAddr, err := sdk.ValAddressFromBech32(vals[0].OperatorAddress)
	suite.Require().NoError(err)

	amt, err := evmAppA.StakingKeeper.GetDelegation(suite.chainA.GetContext(), contractAddrBech32, valAddr)
	suite.Require().NoError(err)
	suite.Require().Equal(sendAmount.BigInt(), amt.Shares.BigInt())

	// Setup rewards for testing
	_, err = suite.prepareStakingRewards(
		suite.chainA.GetContext(),
		stakingRewards{
			Delegator: contractAddrBech32,
			Validator: vals[0],
			RewardAmt: sdkmath.NewInt(RewardAmount),
		},
	)
	suite.Require().NoError(err)
	suite.chainA.NextBlock()

	// Verify minted balance
	stateDB = statedb.New(suite.chainA.GetContext(), evmAppA.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	ethRes, err := evmAppA.GetEVMKeeper().CallEVM(
		suite.chainA.GetContext(),
		stateDB,
		contractData.ABI,
		common.BytesToAddress(senderAddr),
		contractAddr,
		false,
		false,
		nil,
		"balanceOf",
		common.BytesToAddress(senderAddr),
	)
	var bal *big.Int
	err = contractData.ABI.UnpackIntoInterface(&bal, "balanceOf", ethRes.Ret)
	suite.Require().NoError(err)
	suite.Require().Equal(big.NewInt(InitialTokenAmount), bal, "unexpected ERC20 balance")
}

func (suite *ICS20RecursivePrecompileCallsTestSuite) SetupTest() {
	suite.coordinator = evmibctesting.NewCoordinator(suite.T(), 2, 0, integration.SetupEvmd)
	suite.chainA = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
	suite.chainB = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(2))

	evmAppA := suite.chainA.App.(*evmd.EVMD)
	suite.chainAPrecompile = ics20.NewPrecompile(
		evmAppA.BankKeeper,
		*evmAppA.StakingKeeper,
		evmAppA.TransferKeeper,
		evmAppA.IBCKeeper.ChannelKeeper,
		evmAppA.Erc20Keeper,
	)
	bondDenom, err := evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
	suite.Require().NoError(err)

	evmAppA.Erc20Keeper.GetTokenPair(suite.chainA.GetContext(), evmAppA.Erc20Keeper.GetTokenPairID(suite.chainA.GetContext(), bondDenom))
	// evmAppA.Erc20Keeper.SetNativePrecompile(suite.chainA.GetContext(), werc20.Address())

	avail := evmAppA.Erc20Keeper.IsNativePrecompileAvailable(suite.chainA.GetContext(), common.HexToAddress("0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"))
	suite.Require().True(avail)

	evmAppB := suite.chainB.App.(*evmd.EVMD)
	suite.chainBPrecompile = ics20.NewPrecompile(
		evmAppB.BankKeeper,
		*evmAppB.StakingKeeper,
		evmAppB.TransferKeeper,
		evmAppB.IBCKeeper.ChannelKeeper,
		evmAppB.Erc20Keeper,
	)
}

// Constructs the following sends based on the established channels/connections
// 1 - from evmChainA to chainB
func (suite *ICS20RecursivePrecompileCallsTestSuite) TestHandleMsgTransfer() {
	var (
		sourceDenomToTransfer string
		msgAmount             sdkmath.Int
		err                   error
		nativeErc20           *NativeErc20Info
		erc20                 bool
	)

	// originally a basic test case from the IBC testing package, and it has been added as-is to ensure that
	// it still works properly when invoked through the ics20 precompile.
	testCases := []struct {
		name      string
		malleate  func(senderAcc evmibctesting.SenderAccount)
		postCheck func(querier distributionkeeper.Querier, valAddr string, eventAmount int)
	}{
		{
			"test recursive precompile call with reverts",
			func(senderAcc evmibctesting.SenderAccount) {
				// Deploy recursive ERC20 contract with _beforeTokenTransfer override
				contractData, err := contracts.LoadERC20RecursiveReverting()
				suite.Require().NoError(err)

				deploymentData := testutiltypes.ContractDeploymentData{
					Contract:        contractData,
					ConstructorArgs: []interface{}{"RecursiveRevertingToken", "RRCT", uint8(18)},
				}

				contractAddr, err := DeployContract(suite.T(), suite.chainA, deploymentData)
				suite.chainA.NextBlock()
				suite.Require().NoError(err)

				// Setup contract info and test parameters
				nativeErc20 = &NativeErc20Info{
					ContractAddr: contractAddr,
					ContractAbi:  contractData.ABI,
					Denom:        "erc20:" + contractAddr.Hex(),
					InitialBal:   big.NewInt(InitialTokenAmount),
					Account:      common.BytesToAddress(senderAcc.SenderAccount.GetAddress().Bytes()),
				}

				sourceDenomToTransfer = nativeErc20.Denom
				msgAmount = sdkmath.NewIntFromBigInt(nativeErc20.InitialBal)
				erc20 = true

				// Setup contract for testing
				suite.setupContractForTesting(contractAddr, contractData, senderAcc)
			},
			func(querier distributionkeeper.Querier, valAddr string, eventAmount int) {
				evmAppA := suite.chainA.App.(*evmd.EVMD)
				bondDenom, err := evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
				suite.Require().NoError(err)
				contractBondDenomBalance := evmAppA.BankKeeper.GetBalance(suite.chainA.GetContext(), nativeErc20.ContractAddr.Bytes(), bondDenom)
				suite.Require().Equal(contractBondDenomBalance.Amount, sdkmath.NewInt(0))
				// Check distribution rewards after transfer
				afterRewards, err := querier.DelegationRewards(suite.chainA.GetContext(), &distrtypes.QueryDelegationRewardsRequest{
					DelegatorAddress: utils.Bech32StringFromHexAddress(nativeErc20.ContractAddr.String()),
					ValidatorAddress: valAddr,
				})
				suite.Require().NoError(err)
				suite.Require().Equal(afterRewards.Rewards[0].Amount.String(), ExpectedRewards)
				suite.Require().Equal(eventAmount, 20)
			},
		},
		{
			"test recursive precompile call without reverts",
			func(senderAcc evmibctesting.SenderAccount) {
				// Deploy recursive ERC20 contract with _beforeTokenTransfer override
				contractData, err := contracts.LoadERC20RecursiveNonReverting()
				suite.Require().NoError(err)

				deploymentData := testutiltypes.ContractDeploymentData{
					Contract:        contractData,
					ConstructorArgs: []interface{}{"RecursiveNonRevertingToken", "RNRCT", uint8(18)},
				}

				contractAddr, err := DeployContract(suite.T(), suite.chainA, deploymentData)
				suite.chainA.NextBlock()
				suite.Require().NoError(err)

				// Setup contract info and test parameters
				nativeErc20 = &NativeErc20Info{
					ContractAddr: contractAddr,
					ContractAbi:  contractData.ABI,
					Denom:        "erc20:" + contractAddr.Hex(),
					InitialBal:   big.NewInt(InitialTokenAmount),
					Account:      common.BytesToAddress(senderAcc.SenderAccount.GetAddress().Bytes()),
				}

				sourceDenomToTransfer = nativeErc20.Denom
				msgAmount = sdkmath.NewIntFromBigInt(nativeErc20.InitialBal)
				erc20 = true

				// Setup contract for testing
				suite.setupContractForTesting(contractAddr, contractData, senderAcc)
			},
			func(querier distributionkeeper.Querier, valAddr string, eventAmount int) {
				evmAppA := suite.chainA.App.(*evmd.EVMD)
				bondDenom, err := evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
				suite.Require().NoError(err)
				contractBondDenomBalance := evmAppA.BankKeeper.GetBalance(suite.chainA.GetContext(), nativeErc20.ContractAddr.Bytes(), bondDenom)

				suite.Require().Equal(sdkmath.NewInt(50), contractBondDenomBalance.Amount)

				// Check distribution rewards after transfer
				afterRewards, err := querier.DelegationRewards(suite.chainA.GetContext(), &distrtypes.QueryDelegationRewardsRequest{
					DelegatorAddress: utils.Bech32StringFromHexAddress(nativeErc20.ContractAddr.String()),
					ValidatorAddress: valAddr,
				})
				suite.Require().NoError(err)
				suite.Require().Nil(afterRewards.Rewards)
				suite.Require().Equal(eventAmount, 29) // 20 base events + (1 successful reward claim + 1 send + 1 receive + 1 message + 1 transfer) + 4 empty reward claims
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			pathAToB := evmibctesting.NewTransferPath(suite.chainA, suite.chainB)
			pathAToB.Setup()
			traceAToB := transfertypes.NewHop(pathAToB.EndpointB.ChannelConfig.PortID, pathAToB.EndpointB.ChannelID)

			senderAccount := suite.chainA.SenderAccounts[SenderIndex]
			senderAddr := senderAccount.SenderAccount.GetAddress()

			tc.malleate(senderAccount)

			evmAppA := suite.chainA.App.(*evmd.EVMD)

			// Get balance helper function
			GetBalance := func(addr sdk.AccAddress) sdk.Coin {
				ctx := suite.chainA.GetContext()
				if erc20 {
					balanceAmt := evmAppA.Erc20Keeper.BalanceOf(ctx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, nativeErc20.Account)
					return sdk.Coin{
						Denom:  nativeErc20.Denom,
						Amount: sdkmath.NewIntFromBigInt(balanceAmt),
					}
				}
				return evmAppA.BankKeeper.GetBalance(ctx, addr, sourceDenomToTransfer)
			}

			// Verify initial state
			senderBalance := GetBalance(nativeErc20.ContractAddr.Bytes())
			suite.Require().NoError(err)
			bondDenom, err := evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
			suite.Require().NoError(err)
			contractBondDenomBalance := evmAppA.BankKeeper.GetBalance(suite.chainA.GetContext(), nativeErc20.ContractAddr.Bytes(), bondDenom)
			suite.Require().Equal(contractBondDenomBalance.Amount, sdkmath.NewInt(0))

			// Setup transfer parameters
			timeoutHeight := clienttypes.NewHeight(1, TimeoutHeight)
			originalCoin := sdk.NewCoin(sourceDenomToTransfer, msgAmount)

			// Check distribution rewards before transfer
			querier := distributionkeeper.NewQuerier(evmAppA.DistrKeeper)
			vals, err := evmAppA.StakingKeeper.GetAllValidators(suite.chainA.GetContext())
			suite.Require().NoError(err)

			beforeRewards, err := querier.DelegationRewards(suite.chainA.GetContext(), &distrtypes.QueryDelegationRewardsRequest{
				DelegatorAddress: utils.Bech32StringFromHexAddress(nativeErc20.ContractAddr.String()),
				ValidatorAddress: vals[0].OperatorAddress,
			})
			suite.Require().NoError(err)
			suite.Require().Equal(beforeRewards.Rewards[0].Amount.String(), ExpectedRewards)

			// Execute ICS20 transfer (this triggers the bug)
			data, err := suite.chainAPrecompile.Pack("transfer",
				pathAToB.EndpointA.ChannelConfig.PortID,
				pathAToB.EndpointA.ChannelID,
				originalCoin.Denom,
				originalCoin.Amount.BigInt(),
				common.BytesToAddress(senderAddr.Bytes()),        // source addr should be evm hex addr
				suite.chainB.SenderAccount.GetAddress().String(), // receiver should be cosmos bech32 addr
				timeoutHeight,
				uint64(0),
				"",
			)
			suite.Require().NoError(err)

			res, _, _, err := suite.chainA.SendEvmTx(senderAccount, SenderIndex, suite.chainAPrecompile.Address(), big.NewInt(0), data, 0)
			suite.Require().NoError(err) // message committed
			packet, err := evmibctesting.ParsePacketFromEvents(res.Events)
			suite.Require().NoError(err)

			eventAmount := len(res.Events)

			tc.postCheck(querier, vals[0].OperatorAddress, eventAmount)

			// Get the packet data to determine the amount of tokens being transferred (needed for sending entire balance)
			packetData, err := transfertypes.UnmarshalPacketData(packet.GetData(), pathAToB.EndpointA.GetChannel().Version, "")
			suite.Require().NoError(err)
			transferAmount, ok := sdkmath.NewIntFromString(packetData.Token.Amount)
			suite.Require().True(ok)

			afterSenderBalance := GetBalance(senderAddr)
			suite.Require().Equal(
				senderBalance.Amount.Sub(transferAmount).String(),
				afterSenderBalance.Amount.String(),
			)
			if msgAmount == transfertypes.UnboundedSpendLimit() {
				suite.Require().Equal("0", afterSenderBalance.Amount.String(), "sender should have no balance left")
			}

			relayerAddr := suite.chainA.SenderAccounts[0].SenderAccount.GetAddress()
			relayerBalance := GetBalance(relayerAddr)

			// relay send
			pathAToB.EndpointA.Chain.SenderAccount = evmAppA.AccountKeeper.GetAccount(suite.chainA.GetContext(), relayerAddr) // update account in the path as the sequence recorded in that object is out of date
			err = pathAToB.RelayPacket(packet)
			suite.Require().NoError(err) // relay committed

			feeAmt := evmibctesting.FeeCoins().AmountOf(sourceDenomToTransfer)

			// One for UpdateClient() and one for AcknowledgePacket()
			relayPacketFeeAmt := feeAmt.Mul(sdkmath.NewInt(2))

			afterRelayerBalance := GetBalance(relayerAddr)
			suite.Require().Equal(
				relayerBalance.Amount.Sub(relayPacketFeeAmt).String(),
				afterRelayerBalance.Amount.String(),
			)

			escrowAddress := transfertypes.GetEscrowAddress(packet.GetSourcePort(), packet.GetSourceChannel())

			// check that module account escrow address has locked the tokens
			chainAEscrowBalance := evmAppA.BankKeeper.GetBalance(
				suite.chainA.GetContext(),
				escrowAddress,
				sourceDenomToTransfer,
			)
			suite.Require().Equal(transferAmount.String(), chainAEscrowBalance.Amount.String())

			// check that voucher exists on chain B
			evmAppB := suite.chainB.App.(*evmd.EVMD)
			chainBDenom := transfertypes.NewDenom(originalCoin.Denom, traceAToB)
			chainBBalance := evmAppB.BankKeeper.GetBalance(
				suite.chainB.GetContext(),
				suite.chainB.SenderAccount.GetAddress(),
				chainBDenom.IBCDenom(),
			)
			coinSentFromAToB := sdk.NewCoin(chainBDenom.IBCDenom(), transferAmount)
			suite.Require().Equal(coinSentFromAToB, chainBBalance)
		})
	}
}

// TestContractICS20TransferWithDelegationHook tests a contract calling ICS20 transfer
// on an ERC20 token that has a delegation in the beforeTransfer hook
// Contract -> ICS20 Start -> ERC20 -> Delegate -> ICS20 End
func (suite *ICS20RecursivePrecompileCallsTestSuite) TestContractICS20TransferWithDelegationHook() {
	suite.SetupTest() // reset

	pathAToB := evmibctesting.NewTransferPath(suite.chainA, suite.chainB)
	pathAToB.Setup()

	senderAccount := suite.chainA.SenderAccounts[SenderIndex]

	// Deploy ICS20TransferTester contract
	testerData, err := contracts.LoadICS20TransferTester()
	suite.Require().NoError(err)

	testerDeploymentData := testutiltypes.ContractDeploymentData{
		Contract:        testerData,
		ConstructorArgs: []interface{}{},
	}

	testerAddr, err := DeployContract(suite.T(), suite.chainA, testerDeploymentData)
	suite.chainA.NextBlock()
	suite.Require().NoError(err)

	// Deploy regular ERC20 token (no hooks) for dummy transfer
	regularTokenData := contracts.ERC20MinterBurnerDecimalsContract

	regularTokenDeploymentData := testutiltypes.ContractDeploymentData{
		Contract:        regularTokenData,
		ConstructorArgs: []interface{}{"DummyToken", "DT", uint8(18)},
	}

	regularTokenAddr, err := DeployContract(suite.T(), suite.chainA, regularTokenDeploymentData)
	suite.chainA.NextBlock()
	suite.Require().NoError(err)

	// Deploy ERC20WithNativeTransfers contract
	hookTokenData, err := contracts.LoadERC20WithNativeTransfers()
	suite.Require().NoError(err)

	hookTokenDeploymentData := testutiltypes.ContractDeploymentData{
		Contract:        hookTokenData,
		ConstructorArgs: []interface{}{"HookToken", "HT", uint8(18)},
	}

	hookTokenAddr, err := DeployContract(suite.T(), suite.chainA, hookTokenDeploymentData)
	suite.chainA.NextBlock()
	suite.Require().NoError(err)

	evmAppA := suite.chainA.App.(*evmd.EVMD)

	// Register both ERC20 contracts
	_, err = evmAppA.Erc20Keeper.RegisterERC20(suite.chainA.GetContext(), &erc20types.MsgRegisterERC20{
		Signer:         evmAppA.AccountKeeper.GetModuleAddress("gov").String(),
		Erc20Addresses: []string{hookTokenAddr.Hex()},
	})
	suite.Require().NoError(err, "registering hook token should succeed")
	suite.chainA.NextBlock()

	// Mint tokens to tester contract using CallEVM (setup function)
	hookTokenAmount := big.NewInt(InitialTokenAmount)
	mintData, err := hookTokenData.ABI.Pack("mint", testerAddr, hookTokenAmount)
	suite.Require().NoError(err)

	stateDB := statedb.New(suite.chainA.GetContext(), evmAppA.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	deployer := common.BytesToAddress(suite.chainA.SenderPrivKey.PubKey().Address().Bytes())
	_, err = evmAppA.GetEVMKeeper().CallEVMWithData(suite.chainA.GetContext(), stateDB, deployer, &hookTokenAddr, mintData, true, false, nil)
	suite.Require().NoError(err)
	suite.chainA.NextBlock()

	// Mint regular tokens to tester for the dummy transfer
	regularTokenAmount := big.NewInt(1000)
	stateDB = statedb.New(suite.chainA.GetContext(), evmAppA.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	_, err = evmAppA.GetEVMKeeper().CallEVM(
		suite.chainA.GetContext(),
		stateDB,
		regularTokenData.ABI,
		deployer,
		regularTokenAddr,
		true,
		false,
		nil,
		"mint",
		testerAddr,
		regularTokenAmount,
	)
	suite.Require().NoError(err, "mint regular tokens to tester should succeed")
	suite.chainA.NextBlock()

	// Configure hook to perform delegation
	bondDenom, err := evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
	suite.Require().NoError(err)

	vals, err := evmAppA.StakingKeeper.GetAllValidators(suite.chainA.GetContext())
	suite.Require().NoError(err)
	validatorAddr := vals[0].OperatorAddress

	// Fund hook token contract with native tokens for delegation using EVM transaction
	hookTokenAddrSDK := sdk.AccAddress(hookTokenAddr.Bytes())
	delegationAmountSDK := sdkmath.NewInt(DelegationAmount)
	// Fund exact amount: delegation + 2 native transfers (1 aatom each)
	fundAmountAatom := delegationAmountSDK.AddRaw(2) // +2 for two 1 aatom native transfers

	// Send EVM transaction with value to fund the contract (updates both bank module and StateDB)
	_, _, _, err = suite.chainA.SendEvmTx(
		senderAccount,
		0,             // senderAccIdx
		hookTokenAddr, // to (not pointer)
		big.NewInt(fundAmountAatom.Int64()),
		[]byte{}, // empty calldata
		0,        // gasLimit (0 = auto)
	)
	suite.Require().NoError(err)
	suite.chainA.NextBlock()

	// Configure hook parameters
	recipient1 := common.BytesToAddress(senderAccount.SenderAccount.GetAddress().Bytes())
	recipient2 := recipient1
	transferAmount := big.NewInt(1)
	delegateAmount := big.NewInt(DelegationAmount) // Already in base denom

	configData, err := hookTokenData.ABI.Pack(
		"configureHook",
		recipient1,
		recipient2,
		transferAmount,
		validatorAddr,
		delegateAmount,
		true, // enable hook
	)
	suite.Require().NoError(err)

	// Use CallEVM for configuration (setup function)
	stateDB = statedb.New(suite.chainA.GetContext(), evmAppA.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	_, err = evmAppA.GetEVMKeeper().CallEVMWithData(suite.chainA.GetContext(), stateDB, deployer, &hookTokenAddr, configData, true, false, nil)
	suite.Require().NoError(err)
	suite.chainA.NextBlock()

	// Setup transfer parameters
	timeoutHeight := clienttypes.NewHeight(1, TimeoutHeight)
	hookTokenDenom := "erc20:" + hookTokenAddr.Hex()
	transferTokenAmount := sdkmath.NewIntFromBigInt(big.NewInt(InitialTokenAmount / 2))

	// Get balances before
	testerHookBalBefore := evmAppA.Erc20Keeper.BalanceOf(suite.chainA.GetContext(), hookTokenData.ABI, hookTokenAddr, testerAddr)
	hookTokenNativeBalBefore := evmAppA.GetBankKeeper().GetBalance(suite.chainA.GetContext(), hookTokenAddrSDK, bondDenom)
	recipientAddrSDK := senderAccount.SenderAccount.GetAddress()
	recipientNativeBalBefore := evmAppA.GetBankKeeper().GetBalance(suite.chainA.GetContext(), recipientAddrSDK, bondDenom)

	// Call scenario9_transferICS20Transfer from tester contract
	callData, err := testerData.ABI.Pack(
		"scenario9_transferICS20Transfer",
		regularTokenAddr,                                 // token (regular token without hooks)
		recipient1,                                       // recipient for dummy transfer
		big.NewInt(100),                                  // transferAmount (dummy transfer to avoid triggering hookToken hook)
		pathAToB.EndpointA.ChannelConfig.PortID,          // sourcePort
		pathAToB.EndpointA.ChannelID,                     // sourceChannel
		hookTokenDenom,                                   // denom
		transferTokenAmount.BigInt(),                     // ics20Amount
		suite.chainB.SenderAccount.GetAddress().String(), // ics20Receiver
		timeoutHeight,                                    // timeoutHeight
		uint64(0),                                        // timeoutTimestamp
	)
	suite.Require().NoError(err)

	// Execute transaction
	res, _, _, err := suite.chainA.SendEvmTx(senderAccount, SenderIndex, testerAddr, big.NewInt(0), callData, 0)
	suite.Require().NoError(err)

	// Get balances after
	testerHookBalAfter := evmAppA.Erc20Keeper.BalanceOf(suite.chainA.GetContext(), hookTokenData.ABI, hookTokenAddr, testerAddr)
	hookTokenNativeBalAfter := evmAppA.GetBankKeeper().GetBalance(suite.chainA.GetContext(), hookTokenAddrSDK, bondDenom)
	recipientNativeBalAfter := evmAppA.GetBankKeeper().GetBalance(suite.chainA.GetContext(), recipientAddrSDK, bondDenom)

	// Verify ERC20 balance changes
	expectedERC20Delta := transferTokenAmount.BigInt()
	actualERC20Delta := new(big.Int).Sub(testerHookBalBefore, testerHookBalAfter)
	suite.Require().Equal(expectedERC20Delta.String(), actualERC20Delta.String(), "hook token should be transferred via ICS20")

	// Verify native balance changes
	// Hook contract should lose: delegation (1e6 aatom) + 2 native transfers (1 aatom each) = 1_000_002 aatom
	expectedDelegationAatom := DelegationAmount
	expectedTransferAatom := int64(2)
	expectedHookNativeDelta := sdkmath.NewInt(expectedDelegationAatom + expectedTransferAatom)
	actualHookNativeDelta := hookTokenNativeBalBefore.Amount.Sub(hookTokenNativeBalAfter.Amount)
	suite.Require().Equal(expectedHookNativeDelta.String(), actualHookNativeDelta.String(),
		"hook token contract should lose delegation + 2 native transfers")

	// Recipient should receive 2 native transfers (2 aatom)
	expectedRecipientDelta := sdkmath.NewInt(expectedTransferAatom)
	actualRecipientDelta := recipientNativeBalAfter.Amount.Sub(recipientNativeBalBefore.Amount)
	suite.Require().Equal(expectedRecipientDelta.String(), actualRecipientDelta.String(),
		"recipient should receive 2 native transfers")

	// Verify delegation occurred
	delegations, err := evmAppA.StakingKeeper.GetAllDelegatorDelegations(suite.chainA.GetContext(), hookTokenAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(1, len(delegations), "should have 1 delegation from beforeTransfer hook")

	// Verify total bonded amount
	bondedTokens, err := evmAppA.StakingKeeper.GetDelegatorBonded(suite.chainA.GetContext(), hookTokenAddrSDK)
	suite.Require().NoError(err)
	expectedBondedAmount := DelegationAmount // Convert wei to base denom
	suite.Require().Equal(int64(expectedBondedAmount), bondedTokens.Int64(),
		"bonded tokens should equal delegation amount")

	// Verify event count
	suite.Require().Equal(DelegationEventCount+ICS20WithConversionEventCount+EVMEventCount, len(res.Events), "should have 41 events")
}

// TestContractICS20TransferRevertWithDelegationHook tests a contract with reverted ERC20 transfer
// followed by ICS20 transfer on an ERC20 token that has a delegation in the beforeTransfer hook
func (suite *ICS20RecursivePrecompileCallsTestSuite) TestContractICS20TransferRevertWithDelegationHook() {
	suite.SetupTest() // reset

	pathAToB := evmibctesting.NewTransferPath(suite.chainA, suite.chainB)
	pathAToB.Setup()

	senderAccount := suite.chainA.SenderAccounts[SenderIndex]

	// Deploy ICS20TransferTester contract
	testerData, err := contracts.LoadICS20TransferTester()
	suite.Require().NoError(err)

	testerDeploymentData := testutiltypes.ContractDeploymentData{
		Contract:        testerData,
		ConstructorArgs: []interface{}{},
	}

	testerAddr, err := DeployContract(suite.T(), suite.chainA, testerDeploymentData)
	suite.chainA.NextBlock()
	suite.Require().NoError(err)

	// Deploy regular ERC20 token for the first transfer (that will revert)
	regularTokenData := contracts.ERC20MinterBurnerDecimalsContract

	regularTokenDeploymentData := testutiltypes.ContractDeploymentData{
		Contract:        regularTokenData,
		ConstructorArgs: []interface{}{"RegularToken", "RT", uint8(18)},
	}

	regularTokenAddr, err := DeployContract(suite.T(), suite.chainA, regularTokenDeploymentData)
	suite.chainA.NextBlock()
	suite.Require().NoError(err)

	// Deploy ERC20WithNativeTransfers contract
	hookTokenData, err := contracts.LoadERC20WithNativeTransfers()
	suite.Require().NoError(err)

	hookTokenDeploymentData := testutiltypes.ContractDeploymentData{
		Contract:        hookTokenData,
		ConstructorArgs: []interface{}{"HookToken", "HT", uint8(18)},
	}

	hookTokenAddr, err := DeployContract(suite.T(), suite.chainA, hookTokenDeploymentData)
	suite.chainA.NextBlock()
	suite.Require().NoError(err)

	evmAppA := suite.chainA.App.(*evmd.EVMD)

	// Get deployer address for CallEVM
	deployer := common.BytesToAddress(suite.chainA.SenderPrivKey.PubKey().Address().Bytes())

	// Register both ERC20 contracts
	_, err = evmAppA.Erc20Keeper.RegisterERC20(suite.chainA.GetContext(), &erc20types.MsgRegisterERC20{
		Signer:         evmAppA.AccountKeeper.GetModuleAddress("gov").String(),
		Erc20Addresses: []string{regularTokenAddr.Hex(), hookTokenAddr.Hex()},
	})
	suite.Require().NoError(err, "registering tokens should succeed")
	suite.chainA.NextBlock()

	// Mint only a small amount of regular tokens to tester using CallEVM (setup function)
	regularTokenAmount := big.NewInt(1000)
	mintRegularData, err := regularTokenData.ABI.Pack("mint", testerAddr, regularTokenAmount)
	suite.Require().NoError(err)

	stateDB := statedb.New(suite.chainA.GetContext(), evmAppA.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	_, err = evmAppA.GetEVMKeeper().CallEVMWithData(suite.chainA.GetContext(), stateDB, deployer, &regularTokenAddr, mintRegularData, true, false, nil)
	suite.Require().NoError(err)
	suite.chainA.NextBlock()

	// Mint tokens to tester contract (hook token) using CallEVM (setup function)
	hookTokenAmount := big.NewInt(InitialTokenAmount)
	mintHookData, err := hookTokenData.ABI.Pack("mint", testerAddr, hookTokenAmount)
	suite.Require().NoError(err)

	stateDB = statedb.New(suite.chainA.GetContext(), evmAppA.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	_, err = evmAppA.GetEVMKeeper().CallEVMWithData(suite.chainA.GetContext(), stateDB, deployer, &hookTokenAddr, mintHookData, true, false, nil)
	suite.Require().NoError(err)
	suite.chainA.NextBlock()

	vals, err := evmAppA.StakingKeeper.GetAllValidators(suite.chainA.GetContext())
	suite.Require().NoError(err)
	validatorAddr := vals[0].OperatorAddress

	// Fund hook token contract with native tokens for delegation using EVM transaction
	hookTokenAddrSDK := sdk.AccAddress(hookTokenAddr.Bytes())
	delegationAmountSDK := sdkmath.NewInt(DelegationAmount) // Convert from wei to base denom
	// Fund exact amount: delegation + 2 native transfers (1 aatom each)
	fundAmountAatom := delegationAmountSDK.AddRaw(2) // +2 for two 1 aatom native transfers

	// Send EVM transaction with value to fund the contract (updates both bank module and StateDB)
	_, _, _, err = suite.chainA.SendEvmTx(
		senderAccount,
		0,             // senderAccIdx
		hookTokenAddr, // to (not pointer)
		big.NewInt(fundAmountAatom.Int64()),
		[]byte{}, // empty calldata
		0,        // gasLimit (0 = auto)
	)
	suite.Require().NoError(err)
	suite.chainA.NextBlock()

	// Configure hook parameters
	recipient1 := common.BytesToAddress(senderAccount.SenderAccount.GetAddress().Bytes())
	recipient2 := recipient1
	transferAmount := big.NewInt(1)
	delegateAmount := big.NewInt(DelegationAmount) // Already in base denom

	configData, err := hookTokenData.ABI.Pack(
		"configureHook",
		recipient1,
		recipient2,
		transferAmount,
		validatorAddr,
		delegateAmount,
		true, // enable hook
	)
	suite.Require().NoError(err)

	// Use CallEVM for configuration (setup function)
	stateDB = statedb.New(suite.chainA.GetContext(), evmAppA.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	_, err = evmAppA.GetEVMKeeper().CallEVMWithData(suite.chainA.GetContext(), stateDB, deployer, &hookTokenAddr, configData, true, false, nil)
	suite.Require().NoError(err)
	suite.chainA.NextBlock()

	// Setup transfer parameters
	timeoutHeight := clienttypes.NewHeight(1, TimeoutHeight)
	hookTokenDenom := "erc20:" + hookTokenAddr.Hex()
	transferTokenAmount := sdkmath.NewIntFromBigInt(big.NewInt(InitialTokenAmount / 2))

	// Get balances before
	bondDenom, err := evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
	suite.Require().NoError(err)
	regularTokenBalBefore := evmAppA.Erc20Keeper.BalanceOf(suite.chainA.GetContext(), regularTokenData.ABI, regularTokenAddr, testerAddr)
	testerHookBalBefore := evmAppA.Erc20Keeper.BalanceOf(suite.chainA.GetContext(), hookTokenData.ABI, hookTokenAddr, testerAddr)
	hookTokenNativeBalBefore := evmAppA.GetBankKeeper().GetBalance(suite.chainA.GetContext(), hookTokenAddrSDK, bondDenom)
	recipientAddrSDK := senderAccount.SenderAccount.GetAddress()
	recipientNativeBalBefore := evmAppA.GetBankKeeper().GetBalance(suite.chainA.GetContext(), recipientAddrSDK, bondDenom)

	// Call scenario10_transferICS20TransferRevert from tester contract
	// First transfer will revert due to excessive amount
	excessiveAmount := big.NewInt(InitialTokenAmount) // More than the minted amount
	callData, err := testerData.ABI.Pack(
		"scenario10_transferICS20TransferRevert",
		regularTokenAddr,                                 // token (will revert)
		recipient1,                                       // recipient
		excessiveAmount,                                  // excessive transferAmount (will revert)
		pathAToB.EndpointA.ChannelConfig.PortID,          // sourcePort
		pathAToB.EndpointA.ChannelID,                     // sourceChannel
		hookTokenDenom,                                   // denom
		transferTokenAmount.BigInt(),                     // ics20Amount
		suite.chainB.SenderAccount.GetAddress().String(), // ics20Receiver
		timeoutHeight,                                    // timeoutHeight
		uint64(0),                                        // timeoutTimestamp
	)
	suite.Require().NoError(err)

	// Execute contract call
	res, _, _, err := suite.chainA.SendEvmTx(senderAccount, SenderIndex, testerAddr, big.NewInt(0), callData, 0)
	suite.Require().NoError(err)

	// Get balances after
	regularTokenBalAfter := evmAppA.Erc20Keeper.BalanceOf(suite.chainA.GetContext(), regularTokenData.ABI, regularTokenAddr, testerAddr)
	testerHookBalAfter := evmAppA.Erc20Keeper.BalanceOf(suite.chainA.GetContext(), hookTokenData.ABI, hookTokenAddr, testerAddr)
	hookTokenNativeBalAfter := evmAppA.GetBankKeeper().GetBalance(suite.chainA.GetContext(), hookTokenAddrSDK, bondDenom)
	recipientNativeBalAfter := evmAppA.GetBankKeeper().GetBalance(suite.chainA.GetContext(), recipientAddrSDK, bondDenom)

	// Verify regular token balance unchanged (transfer reverted)
	suite.Require().Equal(regularTokenBalBefore.String(), regularTokenBalAfter.String(),
		"regular token balance should be unchanged since transfer reverted")

	// Verify hook token balance changed (ICS20 transfer succeeded)
	expectedERC20Delta := transferTokenAmount.BigInt()
	actualERC20Delta := new(big.Int).Sub(testerHookBalBefore, testerHookBalAfter)
	suite.Require().Equal(expectedERC20Delta.String(), actualERC20Delta.String(),
		"hook token should be transferred via ICS20")

	// Verify native balance changes
	// Hook contract should lose: delegation (1e6 aatom) + 2 native transfers (1 aatom each) = 1_000_002 aatom
	expectedDelegationAatom := DelegationAmount
	expectedTransferAatom := int64(2)
	expectedHookNativeDelta := sdkmath.NewInt(expectedDelegationAatom + expectedTransferAatom)
	actualHookNativeDelta := hookTokenNativeBalBefore.Amount.Sub(hookTokenNativeBalAfter.Amount)
	suite.Require().Equal(expectedHookNativeDelta.String(), actualHookNativeDelta.String(),
		"hook token contract should lose delegation + 2 native transfers")

	// Recipient should receive 2 native transfers (2 aatom)
	expectedRecipientDelta := sdkmath.NewInt(expectedTransferAatom)
	actualRecipientDelta := recipientNativeBalAfter.Amount.Sub(recipientNativeBalBefore.Amount)
	suite.Require().Equal(expectedRecipientDelta.String(), actualRecipientDelta.String(),
		"recipient should receive 2 native transfers")

	// Verify delegation occurred (from beforeTransfer hook)
	delegations, err := evmAppA.StakingKeeper.GetAllDelegatorDelegations(suite.chainA.GetContext(), hookTokenAddrSDK)
	suite.Require().NoError(err)
	suite.Require().Equal(1, len(delegations), "should have 1 delegation from beforeTransfer hook")

	// Verify total bonded amount
	bondedTokens, err := evmAppA.StakingKeeper.GetDelegatorBonded(suite.chainA.GetContext(), hookTokenAddrSDK)
	suite.Require().NoError(err)
	expectedBondedAmount := DelegationAmount // Convert wei to base denom
	suite.Require().Equal(int64(expectedBondedAmount), bondedTokens.Int64(),
		"bonded tokens should equal delegation amount")

	// Verify event count
	suite.Require().Equal(DelegationEventCount+ICS20WithConversionEventCount+EVMEventCount, len(res.Events), "should have 41 events")
}

func TestICS20RecursivePrecompileCallsTestSuite(t *testing.T) {
	suite.Run(t, new(ICS20RecursivePrecompileCallsTestSuite))
}
