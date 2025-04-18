// Copied from https://github.com/cosmos/ibc-go/blob/7325bd2b00fd5e33d895770ec31b5be2f497d37a/modules/apps/transfer/transfer_test.go
// Why was this copied?
// This test suite was imported to validate that ExampleChain (an EVM-based chain)
// correctly supports IBC v1 token transfers using ibc-go’s Transfer module logic.
// The test ensures that ics20 precompile transfer (A → B) behave as expected across channels.
package ibc

import (
	"math/big"
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	"github.com/cosmos/evm/evmd"
	evmibctesting "github.com/cosmos/evm/ibc/testing"
	"github.com/cosmos/evm/precompiles/ics20"
	evmante "github.com/cosmos/evm/x/vm/ante"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type ICS20TransferTestSuite struct {
	suite.Suite

	coordinator *evmibctesting.Coordinator

	// testing chains used for convenience and readability
	chainA           *evmibctesting.TestChain
	chainAPrecompile *ics20.Precompile
	chainB           *evmibctesting.TestChain
}

func (suite *ICS20TransferTestSuite) SetupTest() {
	suite.coordinator = evmibctesting.NewCoordinator(suite.T(), 1, 1)
	suite.chainA = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
	suite.chainB = suite.coordinator.GetChain(evmibctesting.GetChainID(2))

	evmAppA := suite.chainA.App.(*evmd.EVMD)
	suite.chainAPrecompile, _ = ics20.NewPrecompile(
		*evmAppA.StakingKeeper,
		evmAppA.TransferKeeper,
		evmAppA.IBCKeeper.ChannelKeeper,
		evmAppA.AuthzKeeper, // TODO: To be deprecated,
		evmAppA.EVMKeeper,
	)
}

// Constructs the following sends based on the established channels/connections
// 1 - from evmChainA to chainB
func (suite *ICS20TransferTestSuite) TestHandleMsgTransfer() {
	var (
		sourceDenomToTransfer string
		msgAmount             sdkmath.Int
		err                   error
		nativeErc20           *NativeErc20Info
		erc20                 bool
	)

	testCases := []struct {
		name     string
		malleate func()
	}{
		{
			"transfer single denom",
			func() {
				evmApp := suite.chainA.App.(*evmd.EVMD)
				sourceDenomToTransfer, err = evmApp.StakingKeeper.BondDenom(suite.chainA.GetContext())
				msgAmount = evmibctesting.DefaultCoinAmount
			},
		},
		{
			"transfer amount larger than int64",
			func() {
				var ok bool
				evmApp := suite.chainA.App.(*evmd.EVMD)
				sourceDenomToTransfer, err = evmApp.StakingKeeper.BondDenom(suite.chainA.GetContext())
				msgAmount, ok = sdkmath.NewIntFromString("9223372036854775808") // 2^63 (one above int64)
				suite.Require().True(ok)
			},
		},
		{
			"transfer entire balance",
			func() {
				evmApp := suite.chainA.App.(*evmd.EVMD)
				sourceDenomToTransfer, err = evmApp.StakingKeeper.BondDenom(suite.chainA.GetContext())
				msgAmount = types.UnboundedSpendLimit()
			},
		},
		{
			"native erc20 case",
			func() {
				nativeErc20 = SetupNativeErc20(suite.T(), suite.chainA)
				sourceDenomToTransfer = nativeErc20.Denom
				msgAmount = sdkmath.NewIntFromBigInt(nativeErc20.InitialBal)
				erc20 = true
			},
		},
		// TODO: add v2 cases, erc20 token case, registered token pair case, after authz dependency deprecated case
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			// setup between evmChainA and chainB
			// NOTE:
			// pathAToB.EndpointA = endpoint on evmChainA
			// pathAToB.EndpointB = endpoint on chainB
			pathAToB := evmibctesting.NewTransferPath(suite.chainA, suite.chainB)
			pathAToB.Setup()
			traceAToB := types.NewHop(pathAToB.EndpointB.ChannelConfig.PortID, pathAToB.EndpointB.ChannelID)

			tc.malleate()

			evmApp := suite.chainA.App.(*evmd.EVMD)

			GetBalance := func() sdk.Coin {
				ctx := suite.chainA.GetContext()
				if erc20 {
					balanceAmt := evmApp.Erc20Keeper.BalanceOf(ctx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, nativeErc20.Account)
					return sdk.Coin{
						Denom:  nativeErc20.Denom,
						Amount: sdkmath.NewIntFromBigInt(balanceAmt),
					}
				}
				return evmApp.BankKeeper.GetBalance(
					ctx,
					suite.chainA.SenderAccount.GetAddress(),
					sourceDenomToTransfer,
				)
			}

			originalBalance := GetBalance()
			suite.Require().NoError(err)

			timeoutHeight := clienttypes.NewHeight(1, 110)
			originalCoin := sdk.NewCoin(sourceDenomToTransfer, msgAmount)
			sourceAddr := common.BytesToAddress(suite.chainA.SenderAccount.GetAddress().Bytes())

			ctx := suite.chainA.GetContext()
			ctx = evmante.BuildEvmExecutionCtx(ctx).
				WithGasMeter(storetypes.NewInfiniteGasMeter())

			data, err := suite.chainAPrecompile.ABI.Pack("transfer",
				pathAToB.EndpointA.ChannelConfig.PortID,
				pathAToB.EndpointA.ChannelID,
				originalCoin.Denom,
				originalCoin.Amount.BigInt(),
				sourceAddr,                                       // source addr should be evm hex addr
				suite.chainB.SenderAccount.GetAddress().String(), // receiver should be cosmos bech32 addr
				timeoutHeight,
				uint64(0),
				"",
			)
			suite.Require().NoError(err)

			res, err := suite.chainA.SendEvmTx(
				suite.chainA.SenderPrivKey, suite.chainAPrecompile.Address(), big.NewInt(0), data)
			suite.Require().NoError(err) // message committed

			packet, err := evmibctesting.ParsePacketFromEvents(res.Events)
			suite.Require().NoError(err)

			// Get the packet data to determine the amount of tokens being transferred (needed for sending entire balance)
			packetData, err := types.UnmarshalPacketData(packet.GetData(), pathAToB.EndpointA.GetChannel().Version, "")
			suite.Require().NoError(err)
			transferAmount, ok := sdkmath.NewIntFromString(packetData.Token.Amount)
			suite.Require().True(ok)

			chainABalanceBeforeRelay := GetBalance()

			// relay send
			err = pathAToB.RelayPacket(packet)
			suite.Require().NoError(err) // relay committed

			escrowAddress := types.GetEscrowAddress(packet.GetSourcePort(), packet.GetSourceChannel())
			// check that the balance for evmChainA is updated
			chainABalance := evmApp.BankKeeper.GetBalance(
				suite.chainA.GetContext(),
				suite.chainA.SenderAccount.GetAddress(),
				originalCoin.Denom,
			)

			suite.Require().True(chainABalanceBeforeRelay.Amount.Equal(chainABalance.Amount))
			suite.Require().True(originalBalance.Amount.Sub(transferAmount).Equal(chainABalance.Amount))

			// check that module account escrow address has locked the tokens
			chainAEscrowBalance := evmApp.BankKeeper.GetBalance(
				suite.chainA.GetContext(),
				escrowAddress,
				originalCoin.Denom,
			)
			suite.Require().True(transferAmount.Equal(chainAEscrowBalance.Amount))

			// check that voucher exists on chain B
			chainBApp := suite.chainB.GetSimApp()
			chainBDenom := types.NewDenom(originalCoin.Denom, traceAToB)
			chainBBalance := chainBApp.BankKeeper.GetBalance(
				suite.chainB.GetContext(),
				suite.chainB.SenderAccount.GetAddress(),
				chainBDenom.IBCDenom(),
			)
			coinSentFromAToB := sdk.NewCoin(chainBDenom.IBCDenom(), transferAmount)
			suite.Require().Equal(coinSentFromAToB, chainBBalance)
		})
	}
}

func TestICS20TransferTestSuite(t *testing.T) {
	suite.Run(t, new(ICS20TransferTestSuite))
}
