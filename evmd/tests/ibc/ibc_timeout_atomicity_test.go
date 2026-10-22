package ibc

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/statedb"
	transfertypes "github.com/cosmos/ibc-go/v11/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v11/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v11/modules/core/04-channel/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TestTimeoutNativeErc20ReconversionFailureE2E relays a real MsgTimeout
// transaction (finite gas limit, full DeliverTx) for an outbound native ERC20
// transfer whose ERC20 transfer reverts at refund time (token paused).
// Expected property: if the reconversion fails, the refunded bank coin stays
// with the sender and the erc20 module account holds no extra coins.
func (suite *MiddlewareTestSuite) TestTimeoutNativeErc20ReconversionFailureE2E() {
	suite.SetupTest()
	nativeErc20 := SetupNativeErc20(suite.T(), suite.evmChainA, suite.evmChainA.SenderAccounts[0])
	evmApp := suite.evmChainA.App.(*evmd.EVMD)
	path := suite.path

	sendAmt := math.NewIntFromBigInt(nativeErc20.InitialBal)
	senderEthAddr := nativeErc20.Account
	sender := sdk.AccAddress(senderEthAddr.Bytes())
	receiver := suite.chainB.SenderAccount.GetAddress()
	moduleAddr := evmApp.AccountKeeper.GetModuleAddress(types.ModuleName)

	selfHeight := clienttypes.GetSelfHeight(suite.chainB.GetContext())
	timeoutHeight := clienttypes.NewHeight(selfHeight.RevisionNumber, selfHeight.RevisionHeight+3)

	seq, err := suite.transferViaPrecompile(
		suite.evmChainA.GetContext(),
		path.EndpointA.ChannelConfig.PortID,
		path.EndpointA.ChannelID,
		sdk.NewCoin(nativeErc20.Denom, sendAmt),
		sender.String(),
		receiver.String(),
		timeoutHeight,
		0,
		"",
	)
	suite.Require().NoError(err)
	suite.evmChainA.NextBlock()

	// Pause the token (deployer holds the pauser role).
	deployer := common.BytesToAddress([]byte("erc20-test-deployer"))
	ctxA := suite.evmChainA.GetContext()
	stateDB := statedb.New(ctxA, evmApp.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	_, err = evmApp.GetEVMKeeper().CallEVM(ctxA, stateDB, nativeErc20.ContractAbi, deployer, nativeErc20.ContractAddr, true, false, nil, "pause")
	suite.Require().NoError(err)
	suite.evmChainA.NextBlock()

	// Let the packet time out on chain B and update the client on chain A.
	suite.coordinator.CommitNBlocks(suite.chainB, 5)
	suite.Require().NoError(path.EndpointA.UpdateClient())

	packetData := transfertypes.NewFungibleTokenPacketData(nativeErc20.Denom, sendAmt.String(), sender.String(), receiver.String(), "")
	packet := channeltypes.NewPacket(
		packetData.GetBytes(), seq,
		path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID,
		path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID,
		timeoutHeight, 0,
	)

	ctxA = suite.evmChainA.GetContext()
	senderCoinBefore := evmApp.BankKeeper.GetBalance(ctxA, sender, nativeErc20.Denom).Amount
	moduleCoinBefore := evmApp.BankKeeper.GetBalance(ctxA, moduleAddr, nativeErc20.Denom).Amount
	senderERC20Before := evmApp.Erc20Keeper.BalanceOf(ctxA, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)

	// A failed CallEVM consumes the full limit of the tx gas meter (shared by
	// the erc20 callback's cached context). On the v1 stack the ibc-callbacks
	// middleware reads state after the erc20 callback returns, so the
	// MsgTimeout tx runs out of gas and all its writes are discarded.
	// Expected property: no partial state is committed.
	res, err := path.EndpointA.TimeoutPacketWithResult(packet)
	if res != nil {
		suite.T().Logf("MsgTimeout tx: code=%d gasWanted=%d gasUsed=%d log=%q", res.Code, res.GasWanted, res.GasUsed, res.Log)
	}
	suite.Require().Error(err)
	suite.Require().Contains(err.Error(), "out of gas")

	ctxA = suite.evmChainA.GetContext()
	commitment := evmApp.IBCKeeper.ChannelKeeper.GetPacketCommitment(ctxA, packet.SourcePort, packet.SourceChannel, packet.Sequence)
	senderCoinAfter := evmApp.BankKeeper.GetBalance(ctxA, sender, nativeErc20.Denom).Amount
	moduleCoinAfter := evmApp.BankKeeper.GetBalance(ctxA, moduleAddr, nativeErc20.Denom).Amount
	senderERC20After := evmApp.Erc20Keeper.BalanceOf(ctxA, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
	suite.T().Logf("err=%v commitmentDeleted=%v senderCoin %s->%s moduleCoin %s->%s senderERC20 %s->%s",
		err, len(commitment) == 0, senderCoinBefore, senderCoinAfter, moduleCoinBefore, moduleCoinAfter, senderERC20Before, senderERC20After)

	suite.Require().NotEmpty(commitment, "packet commitment must remain when the timeout tx fails")
	suite.Require().Equal(senderCoinBefore.String(), senderCoinAfter.String())
	suite.Require().Equal(moduleCoinBefore.String(), moduleCoinAfter.String())
	suite.Require().Equal(senderERC20Before.String(), senderERC20After.String())
}
