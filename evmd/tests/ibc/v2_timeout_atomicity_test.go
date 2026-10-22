package ibc

import (
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/evmd"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/ibc-go/v11/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v11/modules/core/02-client/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v11/modules/core/04-channel/v2/types"
	hostv2 "github.com/cosmos/ibc-go/v11/modules/core/24-host/v2"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TestV2TimeoutNativeErc20ReconversionFailureE2E relays a real IBC v2
// MsgTimeout transaction (finite gas limit, full DeliverTx) for an outbound
// native ERC20 coin transfer whose ERC20 transfer reverts at refund time
// (token paused). Expected property: if the reconversion fails, the refunded
// bank coin stays with the sender and the erc20 module account holds no
// extra coins.
func (suite *TransferTestSuiteV2) TestV2TimeoutNativeErc20ReconversionFailureE2E() {
	suite.SetupTest()
	senderAcc := suite.evmChainA.SenderAccounts[0]
	nativeErc20 := SetupNativeErc20(suite.T(), suite.evmChainA, senderAcc)
	evmApp := suite.evmChainA.App.(*evmd.EVMD)
	path := suite.pathAToB

	sender := senderAcc.SenderAccount.GetAddress()
	senderEthAddr := common.BytesToAddress(sender.Bytes())
	moduleAddr := evmApp.AccountKeeper.GetModuleAddress(erc20types.ModuleName)
	amount := sdkmath.NewIntFromBigInt(nativeErc20.InitialBal)

	// Convert the ERC20 into its bank coin representation.
	_, err := evmApp.Erc20Keeper.ConvertERC20(suite.evmChainA.GetContext(), &erc20types.MsgConvertERC20{
		ContractAddress: nativeErc20.ContractAddr.Hex(),
		Amount:          amount,
		Receiver:        sender.String(),
		Sender:          senderEthAddr.Hex(),
	})
	suite.Require().NoError(err)
	suite.evmChainA.NextBlock()

	timeoutTimestamp := uint64(suite.chainB.GetContext().BlockTime().Add(time.Minute).Unix()) //nolint:gosec // G115
	msg := types.NewMsgTransferWithEncoding(
		types.PortID, path.EndpointA.ClientID,
		sdk.NewCoin(nativeErc20.Denom, amount), sender.String(),
		suite.chainB.SenderAccount.GetAddress().String(), clienttypes.Height{},
		timeoutTimestamp, "", types.EncodingProtobuf,
	)
	sendRes, err := suite.evmChainA.SendMsgsWithSender(senderAcc, msg)
	suite.Require().NoError(err)
	packets, err := path.EndpointA.ParseV2PacketFromEvent(sendRes.Events)
	suite.Require().NoError(err)
	packet := packets[0]

	// Pause the token (deployer holds the pauser role).
	deployer := common.BytesToAddress([]byte("erc20-test-deployer"))
	ctxA := suite.evmChainA.GetContext()
	stateDB := statedb.New(ctxA, evmApp.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	_, err = evmApp.GetEVMKeeper().CallEVM(ctxA, stateDB, nativeErc20.ContractAbi, deployer, nativeErc20.ContractAddr, true, false, nil, "pause")
	suite.Require().NoError(err)
	suite.evmChainA.NextBlock()

	// Let the packet time out on chain B and update the client on chain A.
	suite.coordinator.IncrementTimeBy(2 * time.Minute)
	suite.coordinator.CommitBlock(suite.chainB)
	suite.Require().NoError(path.EndpointA.UpdateClient())

	ctxA = suite.evmChainA.GetContext()
	senderCoinBefore := evmApp.BankKeeper.GetBalance(ctxA, sender, nativeErc20.Denom).Amount
	moduleCoinBefore := evmApp.BankKeeper.GetBalance(ctxA, moduleAddr, nativeErc20.Denom).Amount

	packetKey := hostv2.PacketReceiptKey(packet.DestinationClient, packet.Sequence)
	proof, proofHeight := path.EndpointB.QueryProof(packetKey)
	timeoutMsg := channeltypesv2.NewMsgTimeout(packet, proof, proofHeight, suite.evmChainA.SenderAccount.GetAddress().String())
	res, err := suite.evmChainA.SendMsgs(timeoutMsg)
	if res != nil {
		suite.T().Logf("v2 MsgTimeout tx: code=%d gasWanted=%d gasUsed=%d log=%q", res.Code, res.GasWanted, res.GasUsed, res.Log)
	}
	suite.Require().NoError(err)

	var sawFailed bool
	for _, ev := range res.Events {
		if ev.Type == erc20types.EventTypeFailedConvertERC20 {
			sawFailed = true
		}
	}

	ctxA = suite.evmChainA.GetContext()
	commitment := evmApp.IBCKeeper.ChannelKeeperV2.GetPacketCommitment(ctxA, packet.SourceClient, packet.Sequence)
	senderCoinAfter := evmApp.BankKeeper.GetBalance(ctxA, sender, nativeErc20.Denom).Amount
	moduleCoinAfter := evmApp.BankKeeper.GetBalance(ctxA, moduleAddr, nativeErc20.Denom).Amount
	senderERC20After := evmApp.Erc20Keeper.BalanceOf(ctxA, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
	suite.T().Logf("failedConvertEvent=%v commitmentDeleted=%v senderCoin %s->%s moduleCoin %s->%s senderERC20After=%s",
		sawFailed, len(commitment) == 0, senderCoinBefore, senderCoinAfter, moduleCoinBefore, moduleCoinAfter, senderERC20After)

	suite.Require().True(sawFailed)
	suite.Require().Equal(amount.String(), senderCoinAfter.String(), "refunded bank coin must remain with the sender when reconversion fails")
	suite.Require().Equal(moduleCoinBefore.String(), moduleCoinAfter.String(), "erc20 module must not retain coins after a failed reconversion")
}
