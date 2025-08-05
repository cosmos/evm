// Copied from https://github.com/cosmos/ibc-go/blob/7325bd2b00fd5e33d895770ec31b5be2f497d37a/modules/apps/transfer/transfer_test.go
// Why was this copied?
// This test suite was imported to validate that ExampleChain (an EVM-based chain)
// correctly supports IBC v1 token transfers using ibc-go’s Transfer module logic.
// The test ensures that multi-hop transfers (A → B → C → B) behave as expected across channels.
package ibc

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/evmd/tests/integration"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	"github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type TransferTestSuite struct {
	suite.Suite

	coordinator *evmibctesting.Coordinator

	// testing chains used for convenience and readability
	evmChainA *evmibctesting.TestChain
	chainB    *evmibctesting.TestChain
	chainC    *evmibctesting.TestChain
}

func (suite *TransferTestSuite) SetupTest() {
	suite.coordinator = evmibctesting.NewCoordinator(suite.T(), 1, 2, integration.SetupEvmd)
	suite.evmChainA = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
	suite.chainB = suite.coordinator.GetChain(evmibctesting.GetChainID(2))
	suite.chainC = suite.coordinator.GetChain(evmibctesting.GetChainID(3))
}

// Constructs the following sends based on the established channels/connections
// 1 - from evmChainA to chainB
// 2 - from chainB to chainC
// 3 - from chainC to chainB
func (suite *TransferTestSuite) TestHandleMsgTransfer() {
	var (
		sourceDenomToTransfer string
		msgAmount             sdkmath.Int
		err                   error
	)

	// originally a basic test case from the IBC testing package, and it has been added as-is to ensure that
	// it still works properly when invoked by evmd app.
	testCases := []struct {
		name     string
		malleate func()
	}{
		{
			"transfer single denom",
			func() {
				msgAmount = evmibctesting.DefaultCoinAmount
			},
		},
		{
			"transfer amount larger than int64",
			func() {
				var ok bool
				msgAmount, ok = sdkmath.NewIntFromString("9223372036854775808") // 2^63 (one above int64)
				suite.Require().True(ok)
			},
		},
		{
			"transfer entire balance",
			func() {
				msgAmount = types.UnboundedSpendLimit()
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			// setup between evmChainA and chainB
			// NOTE:
			// pathAToB.EndpointA = endpoint on evmChainA
			// pathAToB.EndpointB = endpoint on chainB
			pathAToB := evmibctesting.NewTransferPath(suite.evmChainA, suite.chainB)
			pathAToB.Setup()
			traceAToB := types.NewHop(pathAToB.EndpointB.ChannelConfig.PortID, pathAToB.EndpointB.ChannelID)

			senderIdx := 1
			senderAccount := suite.evmChainA.SenderAccounts[senderIdx]
			senderAddr := senderAccount.SenderAccount.GetAddress()
			tc.malleate()

			evmApp := suite.evmChainA.App.(*evmd.EVMD)
			sourceDenomToTransfer, err = evmApp.StakingKeeper.BondDenom(suite.evmChainA.GetContext())
			suite.Require().NoError(err)
			senderBalance := evmApp.BankKeeper.GetBalance(suite.evmChainA.GetContext(), senderAddr, sourceDenomToTransfer)

			timeoutHeight := clienttypes.NewHeight(1, 110)

			originalCoin := sdk.NewCoin(sourceDenomToTransfer, msgAmount)

			// send from evmChainA to chainB
			msg := types.NewMsgTransfer(
				pathAToB.EndpointA.ChannelConfig.PortID,
				pathAToB.EndpointA.ChannelID,
				originalCoin,
				senderAddr.String(),
				suite.chainB.SenderAccount.GetAddress().String(),
				timeoutHeight, 0, "",
			)
			fee := evmibctesting.FeeCoins().AmountOf(sourceDenomToTransfer)
			res, err := suite.evmChainA.SendMsgsWithSender(senderAccount, msg)
			suite.Require().NoError(err) // message committed

			packet, err := evmibctesting.ParsePacketFromEvents(res.Events)
			suite.Require().NoError(err)

			// Get the packet data to determine the amount of tokens being transferred (needed for sending entire balance)
			packetData, err := types.UnmarshalPacketData(packet.GetData(), pathAToB.EndpointA.GetChannel().Version, "")
			suite.Require().NoError(err)
			transferAmount, ok := sdkmath.NewIntFromString(packetData.Token.Amount)
			suite.Require().True(ok)

			afterSenderBalance := evmApp.BankKeeper.GetBalance(suite.evmChainA.GetContext(), senderAddr, sourceDenomToTransfer)
			suite.Require().Equal(
				senderBalance.Amount.Sub(fee).Sub(transferAmount).String(),
				afterSenderBalance.Amount.String(),
			)
			if msgAmount == types.UnboundedSpendLimit() {
				suite.Require().Equal("0", afterSenderBalance.Amount.String(), "sender should have no balance left")
			}

			relayerAddr := suite.evmChainA.SenderAccounts[0].SenderAccount.GetAddress()
			relayerBalance := evmApp.BankKeeper.GetBalance(suite.evmChainA.GetContext(), relayerAddr, originalCoin.Denom)

			// relay send
			err = pathAToB.RelayPacket(packet)
			suite.Require().NoError(err) // relay committed

			// One for UpdateClient() and one for AcknowledgePacket()
			relayPacketFeeAmt := fee.Mul(sdkmath.NewInt(2))

			afterRelayerBalance := evmApp.BankKeeper.GetBalance(suite.evmChainA.GetContext(), relayerAddr, originalCoin.Denom)
			suite.Require().Equal(
				relayerBalance.Amount.Sub(relayPacketFeeAmt).String(),
				afterRelayerBalance.Amount.String(),
			)

			escrowAddress := types.GetEscrowAddress(packet.GetSourcePort(), packet.GetSourceChannel())

			// check that module account escrow address has locked the tokens
			chainAEscrowBalance := evmApp.BankKeeper.GetBalance(
				suite.evmChainA.GetContext(),
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

			// setup between chainB to chainC
			// NOTE:
			// pathBToC.EndpointA = endpoint on chainB
			// pathBToC.EndpointB = endpoint on chainC
			pathBToC := evmibctesting.NewTransferPath(suite.chainB, suite.chainC)
			pathBToC.Setup()
			traceBToC := types.NewHop(pathBToC.EndpointB.ChannelConfig.PortID, pathBToC.EndpointB.ChannelID)

			// send from chainB to chainC
			msg = types.NewMsgTransfer(
				pathBToC.EndpointA.ChannelConfig.PortID,
				pathBToC.EndpointA.ChannelID,
				coinSentFromAToB,
				suite.chainB.SenderAccount.GetAddress().String(),
				suite.chainC.SenderAccount.GetAddress().String(),
				timeoutHeight, 0, "",
			)
			res, err = suite.chainB.SendMsgs(msg)
			suite.Require().NoError(err) // message committed

			packet, err = evmibctesting.ParsePacketFromEvents(res.Events)
			suite.Require().NoError(err)

			err = pathBToC.RelayPacket(packet)
			suite.Require().NoError(err) // relay committed

			coinsSentFromBToC := sdk.NewCoins()
			// check balances for chainB and chainC after transfer from chainB to chainC
			// NOTE: fungible token is prefixed with the full trace in order to verify the packet commitment
			chainCDenom := types.NewDenom(originalCoin.Denom, traceBToC, traceAToB)

			// check that the balance is updated on chainC
			chainCApp := suite.chainC.GetSimApp()
			coinSentFromBToC := sdk.NewCoin(chainCDenom.IBCDenom(), transferAmount)
			chainCBalance := chainCApp.BankKeeper.GetBalance(
				suite.chainC.GetContext(),
				suite.chainC.SenderAccount.GetAddress(),
				coinSentFromBToC.Denom,
			)
			suite.Require().Equal(coinSentFromBToC, chainCBalance)

			// check that balance on chain B is empty
			chainBBalance = chainBApp.BankKeeper.GetBalance(
				suite.chainB.GetContext(),
				suite.chainB.SenderAccount.GetAddress(),
				coinSentFromBToC.Denom,
			)
			suite.Require().Zero(chainBBalance.Amount.Int64())

			// send from chainC back to chainB
			msg = types.NewMsgTransfer(
				pathBToC.EndpointB.ChannelConfig.PortID,
				pathBToC.EndpointB.ChannelID, coinSentFromBToC,
				suite.chainC.SenderAccount.GetAddress().String(),
				suite.chainB.SenderAccount.GetAddress().String(),
				timeoutHeight, 0, "",
			)
			res, err = suite.chainC.SendMsgs(msg)
			suite.Require().NoError(err) // message committed

			packet, err = evmibctesting.ParsePacketFromEvents(res.Events)
			suite.Require().NoError(err)

			err = pathBToC.RelayPacket(packet)
			suite.Require().NoError(err) // relay committed

			// check balances for chainC are empty after transfer from chainC to chainB
			for _, coin := range coinsSentFromBToC {
				// check that balance on chain C is empty
				chainCBalance := chainCApp.BankKeeper.GetBalance(
					suite.chainC.GetContext(),
					suite.chainC.SenderAccount.GetAddress(),
					coin.Denom,
				)
				suite.Require().Zero(chainCBalance.Amount.Int64())
			}

			// check balances for chainB after transfer from chainC to chainB
			// check that balance on chain B has the transferred amount
			chainBBalance = chainBApp.BankKeeper.GetBalance(
				suite.chainB.GetContext(),
				suite.chainB.SenderAccount.GetAddress(),
				coinSentFromAToB.Denom,
			)
			suite.Require().Equal(coinSentFromAToB, chainBBalance)

			// check that module account escrow address is empty
			escrowAddress = types.GetEscrowAddress(traceBToC.PortId, traceBToC.ChannelId)
			chainBEscrowBalance := chainBApp.BankKeeper.GetBalance(
				suite.chainB.GetContext(),
				escrowAddress,
				coinSentFromAToB.Denom,
			)
			suite.Require().Zero(chainBEscrowBalance.Amount.Int64())

			// check balances for evmChainA after transfer from chainC to chainB
			// check that the balance is unchanged
			chainASenderBalance := evmApp.BankKeeper.GetBalance(suite.evmChainA.GetContext(), senderAddr, originalCoin.Denom)
			suite.Require().Equal(
				afterSenderBalance.Amount.String(),
				chainASenderBalance.Amount.String(),
			)

			// check that module account escrow address is unchanged
			escrowAddress = types.GetEscrowAddress(pathAToB.EndpointA.ChannelConfig.PortID, pathAToB.EndpointA.ChannelID)
			chainAEscrowBalance = evmApp.BankKeeper.GetBalance(suite.evmChainA.GetContext(), escrowAddress, originalCoin.Denom)
			suite.Require().True(transferAmount.Equal(chainAEscrowBalance.Amount))
		})
	}
}

func TestTransferTestSuite(t *testing.T) {
	suite.Run(t, new(TransferTestSuite))
}
