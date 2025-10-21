// Copied from https://github.com/cosmos/ibc-go/blob/7325bd2b00fd5e33d895770ec31b5be2f497d37a/modules/apps/transfer/transfer_test.go
// Why was this copied?
// This test suite was imported to validate that ExampleChain (an EVM-based chain)
// correctly supports IBC v1 token transfers using ibc-go’s Transfer module logic.
// The test ensures that ics20 precompile transfer (A → B) behave as expected across channels.
package ibc

import (
	"fmt"
	"math/big"
	"testing"

	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/evm/utils"

	"github.com/cosmos/evm/contracts"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/precompiles/ics20"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Test constants
const (
	// Token amounts
	InitialTokenAmount = 1_000_000_000_000_000_000 // 1 token with 18 decimals
	DelegationAmount   = 1_000_000_000_000_000_000 // 1 token for delegation
	RewardAmount       = 100                       // 100 base units for rewards
	ExpectedRewards    = "50.000000000000000000"   // Expected reward amount after allocation

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
	ctxA := suite.chainA.GetContext()
	senderAddr := senderAcc.SenderAccount.GetAddress()
	senderEVMAddr := common.BytesToAddress(senderAddr.Bytes())
	deployerAddr := common.BytesToAddress(suite.chainA.SenderPrivKey.PubKey().Address().Bytes())

	// Register ERC20 contract
	_, err := evmAppA.Erc20Keeper.RegisterERC20(ctxA, &erc20types.MsgRegisterERC20{
		Signer:         evmAppA.AccountKeeper.GetModuleAddress("gov").String(),
		Erc20Addresses: []string{contractAddr.Hex()},
	})
	suite.Require().NoError(err, "registering ERC20 token should succeed")
	suite.chainA.NextBlock()

	// Send native tokens to contract for delegation
	bondDenom, err := evmAppA.StakingKeeper.BondDenom(ctxA)
	suite.Require().NoError(err)

	contractAddrBech32, err := sdk.AccAddressFromHexUnsafe(contractAddr.Hex()[2:])
	suite.Require().NoError(err)

	deployerAddrBech32 := sdk.AccAddress(deployerAddr.Bytes())
	deployerBalance := evmAppA.BankKeeper.GetBalance(ctxA, deployerAddrBech32, bondDenom)

	// Send delegation amount to contract
	sendAmount := sdkmath.NewInt(DelegationAmount)
	if deployerBalance.Amount.LT(sendAmount) {
		sendAmount = deployerBalance.Amount.Quo(sdkmath.NewInt(2))
	}

	err = evmAppA.BankKeeper.SendCoins(
		ctxA,
		deployerAddrBech32,
		contractAddrBech32,
		sdk.NewCoins(sdk.NewCoin(bondDenom, sendAmount)),
	)
	suite.Require().NoError(err, "sending native tokens to contract should succeed")

	// Mint ERC20 tokens
	_, err = evmAppA.GetEVMKeeper().CallEVM(
		suite.chainA.GetContext(),
		contractData.ABI,
		deployerAddr,
		contractAddr,
		true,
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

	_, err = evmAppA.GetEVMKeeper().CallEVM(
		ctxA,
		contractData.ABI,
		deployerAddr,
		contractAddr,
		true,
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
	bal := evmAppA.GetErc20Keeper().BalanceOf(ctxA, contractData.ABI, contractAddr, common.BytesToAddress(senderAddr))
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
	)
	bondDenom, err := evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
	suite.Require().NoError(err)

	evmAppA.Erc20Keeper.GetTokenPair(suite.chainA.GetContext(), evmAppA.Erc20Keeper.GetTokenPairID(suite.chainA.GetContext(), bondDenom))
	//evmAppA.Erc20Keeper.SetNativePrecompile(suite.chainA.GetContext(), werc20.Address())

	avail := evmAppA.Erc20Keeper.IsNativePrecompileAvailable(suite.chainA.GetContext(), common.HexToAddress("0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"))
	suite.Require().True(avail)

	evmAppB := suite.chainB.App.(*evmd.EVMD)
	suite.chainBPrecompile = ics20.NewPrecompile(
		evmAppB.BankKeeper,
		*evmAppB.StakingKeeper,
		evmAppB.TransferKeeper,
		evmAppB.IBCKeeper.ChannelKeeper,
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

				suite.Require().Equal(contractBondDenomBalance.Amount, sdkmath.NewInt(50))

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
			fmt.Println(res.Events)

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
			pathAToB.EndpointA.Chain.SenderAccount = evmAppA.AccountKeeper.GetAccount(suite.chainA.GetContext(), relayerAddr) //update account in the path as the sequence recorded in that object is out of date
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

func TestICS20RecursivePrecompileCallsTestSuite(t *testing.T) {
	suite.Run(t, new(ICS20RecursivePrecompileCallsTestSuite))
}
