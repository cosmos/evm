package ibc

import (
	"bytes"
	"errors"
	"math/big"
	"testing"
	"time"

	testifysuite "github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/testutil"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	erc20Keeper "github.com/cosmos/evm/x/erc20/keeper"
	"github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/erc20/v2"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"
	ibcmockv2 "github.com/cosmos/ibc-go/v10/testing/mock/v2"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MiddlewareTestSuite tests the v2 IBC middleware for the ERC20 module.
type MiddlewareV2TestSuite struct {
	testifysuite.Suite

	coordinator *evmibctesting.Coordinator

	// testing chains used for convenience and readability
	evmChainA *evmibctesting.TestChain
	chainB    *evmibctesting.TestChain

	// evmChainA to chainB for testing OnSendPacket, OnAckPacket, and OnTimeoutPacket
	pathAToB *evmibctesting.Path
	// chainB to evmChainA for testing OnRecvPacket
	pathBToA *evmibctesting.Path
}

func (suite *MiddlewareV2TestSuite) SetupTest() {
	suite.coordinator = evmibctesting.NewCoordinator(suite.T(), 1, 1)
	suite.evmChainA = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
	suite.chainB = suite.coordinator.GetChain(evmibctesting.GetChainID(2))

	// setup between evmChainA and chainB
	// pathAToB.EndpointA = endpoint on evmChainA
	// pathAToB.EndpointB = endpoint on chainB
	suite.pathAToB = evmibctesting.NewPath(suite.evmChainA, suite.chainB)
	// setup between chainB and evmChainA
	// path.EndpointA = endpoint on chainB
	// path.EndpointB = endpoint on evmChainA
	suite.pathBToA = evmibctesting.NewPath(suite.chainB, suite.evmChainA)

	// setup IBC v2 paths between the chains
	suite.pathAToB.SetupV2()
	suite.pathBToA.SetupV2()
}

func TestMiddlewareV2TestSuite(t *testing.T) {
	testifysuite.Run(t, new(MiddlewareV2TestSuite))
}

func (suite *MiddlewareV2TestSuite) TestNewIBCMiddleware() {
	testCases := []struct {
		name          string
		instantiateFn func()
		expError      error
	}{
		{
			"success",
			func() {
				_ = v2.NewIBCMiddleware(ibcmockv2.IBCModule{}, erc20Keeper.Keeper{})
			},
			nil,
		},
		{
			"panics with nil underlying app",
			func() {
				_ = v2.NewIBCMiddleware(nil, erc20Keeper.Keeper{})
			},
			errors.New("underlying application cannot be nil"),
		},
		{
			"panics with nil erc20 keeper",
			func() {
				_ = v2.NewIBCMiddleware(ibcmockv2.IBCModule{}, nil)
			},
			errors.New("erc20 keeper cannot be nil"),
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			if tc.expError == nil {
				suite.Require().NotPanics(
					tc.instantiateFn,
					"unexpected panic: NewIBCMiddleware",
				)
			} else {
				suite.Require().PanicsWithError(
					tc.expError.Error(),
					tc.instantiateFn,
					"expected panic with error: ", tc.expError.Error(),
				)
			}
		})
	}
}

func (suite *MiddlewareV2TestSuite) TestOnSendPacket() {
	var (
		ctx        sdk.Context
		packetData transfertypes.FungibleTokenPacketData
		payload    channeltypesv2.Payload
	)

	testCases := []struct {
		name     string
		malleate func()
		expError string
	}{
		{
			name:     "pass",
			malleate: nil,
			expError: "",
		},
		{
			name: "fail: malformed packet data",
			malleate: func() {
				payload.Value = []byte("malformed")
			},
			expError: "cannot unmarshal ICS20-V1 transfer packet data",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx = suite.evmChainA.GetContext()
			evmApp := suite.evmChainA.App.(*evmd.EVMD)
			bondDenom, err := evmApp.StakingKeeper.BondDenom(ctx)
			suite.Require().NoError(err)
			packetData = transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				ibctesting.DefaultCoinAmount.String(),
				suite.evmChainA.SenderAccount.GetAddress().String(),
				suite.chainB.SenderAccount.GetAddress().String(),
				"",
			)

			payload = channeltypesv2.NewPayload(
				transfertypes.PortID, transfertypes.PortID,
				transfertypes.V1, transfertypes.EncodingJSON,
				packetData.GetBytes(),
			)

			if tc.malleate != nil {
				tc.malleate()
			}

			onSendPacket := func() error {
				return evmApp.GetIBCKeeper().ChannelKeeperV2.Router.Route(ibctesting.TransferPort).OnSendPacket(
					ctx,
					suite.pathAToB.EndpointA.ClientID,
					suite.pathAToB.EndpointB.ClientID,
					1,
					payload,
					suite.evmChainA.SenderAccount.GetAddress(),
				)
			}

			err = onSendPacket()
			if tc.expError != "" {
				suite.Require().Error(err)
				suite.Require().ErrorContains(err, tc.expError)
			} else {
				suite.Require().NoError(err)
				// check that the escrowed coins are in the escrow account
				escrowAddress := transfertypes.GetEscrowAddress(
					transfertypes.PortID,
					suite.pathAToB.EndpointA.ClientID,
				)
				escrowedCoins := evmApp.BankKeeper.GetAllBalances(ctx, escrowAddress)
				suite.Require().Equal(1, len(escrowedCoins))
				suite.Require().Equal(ibctesting.DefaultCoinAmount.String(), escrowedCoins[0].Amount.String())
				suite.Require().Equal(bondDenom, escrowedCoins[0].Denom)
			}
		})
	}
}

func (suite *MiddlewareV2TestSuite) TestOnRecvPacket() {
	var (
		ctx        sdk.Context
		packetData transfertypes.FungibleTokenPacketData
		payload    channeltypesv2.Payload
	)

	testCases := []struct {
		name      string
		malleate  func()
		expResult channeltypesv2.PacketStatus
	}{
		{
			name:      "pass",
			malleate:  nil,
			expResult: channeltypesv2.PacketStatus_Success,
		},
		{
			name: "fail: malformed packet data",
			malleate: func() {
				payload.Value = []byte("malformed")
			},
			expResult: channeltypesv2.PacketStatus_Failure,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx = suite.chainB.GetContext()
			bondDenom, err := suite.chainB.GetSimApp().StakingKeeper.BondDenom(ctx)
			suite.Require().NoError(err)
			receiver := suite.evmChainA.SenderAccount.GetAddress()
			sendAmt := ibctesting.DefaultCoinAmount
			packetData = transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				sendAmt.String(),
				suite.chainB.SenderAccount.GetAddress().String(),
				receiver.String(),
				"",
			)

			payload = channeltypesv2.NewPayload(
				transfertypes.PortID, transfertypes.PortID,
				transfertypes.V1, transfertypes.EncodingJSON,
				packetData.GetBytes(),
			)

			if tc.malleate != nil {
				tc.malleate()
			}

			evmApp := suite.evmChainA.App.(*evmd.EVMD)
			// erc20 module is routed as top level middleware
			transferStack := evmApp.GetIBCKeeper().ChannelKeeperV2.Router.Route(ibctesting.TransferPort)
			sourceClient := suite.pathBToA.EndpointB.ClientID
			onRecvPacket := func() channeltypesv2.RecvPacketResult {
				ctx = suite.evmChainA.GetContext()
				return transferStack.OnRecvPacket(
					ctx,
					sourceClient,
					suite.pathBToA.EndpointA.ClientID,
					1,
					payload,
					receiver,
				)
			}

			recvResult := onRecvPacket()
			suite.Require().Equal(tc.expResult, recvResult.Status)
			if recvResult.Status == channeltypesv2.PacketStatus_Success {
				// make sure voucher coins are sent to the receiver
				data, ackErr := transfertypes.UnmarshalPacketData(packetData.GetBytes(), transfertypes.V1, "")
				suite.Require().Nil(ackErr)
				voucherDenom := testutil.GetVoucherDenomFromPacketData(data, payload.GetSourcePort(), sourceClient)
				voucherCoin := evmApp.BankKeeper.GetBalance(ctx, receiver, voucherDenom)
				suite.Require().Equal(sendAmt.String(), voucherCoin.Amount.String())
				// make sure token pair is registered
				singleTokenRepresentation, err := types.NewTokenPairSTRv2(voucherDenom)
				suite.Require().NoError(err)
				tokenPair, found := evmApp.Erc20Keeper.GetTokenPair(ctx, singleTokenRepresentation.GetID())
				suite.Require().True(found)
				suite.Require().Equal(voucherDenom, tokenPair.Denom)
				// Make sure dynamic precompile is registered
				params := evmApp.Erc20Keeper.GetParams(ctx)
				suite.Require().Contains(params.DynamicPrecompiles, tokenPair.Erc20Address)
			}
		})
	}
}

// TestOnRecvPacketNativeERC20 tests the OnRecvPacket logic when the packet involves a native ERC20.
func (suite *MiddlewareV2TestSuite) TestOnRecvPacketNativeERC20() {
	var (
		packetData transfertypes.FungibleTokenPacketData
		payload    channeltypesv2.Payload
	)

	testCases := []struct {
		name      string
		malleate  func()
		expResult channeltypesv2.PacketStatus
	}{
		{
			name:      "pass",
			malleate:  nil,
			expResult: channeltypesv2.PacketStatus_Success,
		},
		{
			name: "fail: malformed packet data",
			malleate: func() {
				payload.Value = []byte("malformed")
			},
			expResult: channeltypesv2.PacketStatus_Failure,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			nativeErc20 := SetupNativeErc20(suite.T(), suite.evmChainA)
			senderEthAddr := nativeErc20.Account
			sender := sdk.AccAddress(senderEthAddr.Bytes())
			sendAmt := math.NewIntFromBigInt(nativeErc20.InitialBal)

			evmCtx := suite.evmChainA.GetContext()
			evmApp := suite.evmChainA.App.(*evmd.EVMD)
			// MOCK erc20 native coin transfer from chainA to chainB
			// 1: Convert erc20 tokens to native erc20 coins for sending through IBC.
			_, err := evmApp.Erc20Keeper.ConvertERC20(
				evmCtx,
				types.NewMsgConvertERC20(
					sendAmt,
					sender,
					nativeErc20.ContractAddr,
					senderEthAddr,
				),
			)
			suite.Require().NoError(err)
			// 1-1: Check native erc20 token is converted to native erc20 coin on chainA.
			erc20BalAfterConvert := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
			suite.Require().Equal(
				new(big.Int).Sub(nativeErc20.InitialBal, sendAmt.BigInt()).String(),
				erc20BalAfterConvert.String(),
			)
			balAfterConvert := evmApp.BankKeeper.GetBalance(evmCtx, sender, nativeErc20.Denom)
			suite.Require().Equal(sendAmt.String(), balAfterConvert.Amount.String())

			// 2: Transfer erc20 native coin to chainB through IBC.
			path := suite.pathAToB
			chainBAcc := suite.chainB.SenderAccount.GetAddress()
			packetData = transfertypes.NewFungibleTokenPacketData(
				nativeErc20.Denom, sendAmt.String(),
				sender.String(), chainBAcc.String(),
				"",
			)
			payload = channeltypesv2.NewPayload(
				transfertypes.PortID, transfertypes.PortID,
				transfertypes.V1, transfertypes.EncodingJSON,
				packetData.GetBytes(),
			)
			timeoutTimestamp := uint64(suite.chainB.GetContext().BlockTime().Add(time.Hour).Unix()) //nolint:gosec // G115
			channelKeeperV2 := evmApp.GetIBCKeeper().ChannelKeeperV2
			_, err = channelKeeperV2.SendPacket(evmCtx, channeltypesv2.NewMsgSendPacket(
				path.EndpointA.ClientID,
				timeoutTimestamp,
				sender.String(),
				payload,
			))
			suite.Require().NoError(err)
			// 2-1: Check native erc20 token is escrowed on evmChainA for sending to chainB.
			escrowAddr := transfertypes.GetEscrowAddress(transfertypes.PortID, path.EndpointA.ClientID)
			escrowedBal := evmApp.BankKeeper.GetBalance(evmCtx, escrowAddr, nativeErc20.Denom)
			suite.Require().Equal(sendAmt.String(), escrowedBal.Amount.String())

			// 3: Assume chainB received native erc20 coin from evmChain A
			// 3-1: Mock sending back from chainB to chainA
			// chainBNativeErc20Denom is the native erc20 token denom on chainB from evmChainA through IBC.
			chainBNativeErc20Denom := transfertypes.NewDenom(
				nativeErc20.Denom,
				transfertypes.NewHop(
					transfertypes.PortID,
					path.EndpointB.ClientID,
				),
			)
			receiver := suite.evmChainA.SenderAccount.GetAddress()
			packetData = transfertypes.NewFungibleTokenPacketData(
				chainBNativeErc20Denom.Path(), sendAmt.String(),
				chainBAcc.String(), receiver.String(), "",
			)
			payload = channeltypesv2.NewPayload(
				transfertypes.PortID, transfertypes.PortID,
				transfertypes.V1, transfertypes.EncodingJSON,
				packetData.GetBytes(),
			)

			if tc.malleate != nil {
				tc.malleate()
			}

			onRecvPacket := func() channeltypesv2.RecvPacketResult {
				return channelKeeperV2.Router.Route(ibctesting.TransferPort).OnRecvPacket(
					evmCtx,
					path.EndpointB.ClientID,
					path.EndpointA.ClientID,
					1,
					payload,
					receiver,
				)
			}
			// 4: Packet is received on evmChainA from chainB
			recvResult := onRecvPacket()
			suite.Require().Equal(tc.expResult, recvResult.Status)
			if recvResult.Status == channeltypesv2.PacketStatus_Success {
				// Check un-escrowed balance on evmChainA after receiving the packet.
				escrowedBal = evmApp.BankKeeper.GetBalance(evmCtx, escrowAddr, nativeErc20.Denom)
				suite.Require().True(escrowedBal.IsZero(), "escrowed balance should be un-escrowed after receiving the packet")
				balAfterUnescrow := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
				suite.Require().Equal(nativeErc20.InitialBal.String(), balAfterUnescrow.String())
				bankBalAfterUnescrow := evmApp.BankKeeper.GetBalance(evmCtx, sender, nativeErc20.Denom)
				suite.Require().True(bankBalAfterUnescrow.IsZero(), "no duplicate state in the bank balance")
			}
		})
	}
}

func (suite *MiddlewareV2TestSuite) TestOnAcknowledgementPacket() {
	var (
		ctx        sdk.Context
		packetData transfertypes.FungibleTokenPacketData
		ack        []byte
		payload    channeltypesv2.Payload
	)

	testCases := []struct {
		name           string
		malleate       func()
		onSendRequired bool
		expError       string
	}{
		{
			name:           "pass",
			malleate:       nil,
			onSendRequired: false,
			expError:       "",
		},
		{
			name: "pass: refund escrowed token because ack err(UNIVERSAL_ERROR_ACKNOWLEDGEMENT)",
			malleate: func() {
				ack = channeltypesv2.ErrorAcknowledgement[:]
			},
			onSendRequired: true, // this test case handles the refund of the escrowed token, so we need to call OnSendPacket.
			expError:       "",
		},
		{
			name: "fail: malformed packet data",
			malleate: func() {
				payload.Value = []byte("malformed")
			},
			onSendRequired: false,
			expError:       "cannot unmarshal ICS20-V1 transfer packet data",
		},
		{
			name: "fail: empty ack",
			malleate: func() {
				ack = []byte{}
			},
			onSendRequired: false,
			expError:       "cannot unmarshal ICS-20 transfer packet acknowledgement",
		},
		{
			name: "fail: ack error",
			malleate: func() {
				ackErr := channeltypes.NewErrorAcknowledgement(errors.New("error"))
				ack = ackErr.Acknowledgement()
			},
			onSendRequired: false,
			expError:       "cannot pass in a custom error acknowledgement with IBC v2",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx = suite.evmChainA.GetContext()
			evmApp := suite.evmChainA.App.(*evmd.EVMD)
			bondDenom, err := evmApp.StakingKeeper.BondDenom(ctx)
			suite.Require().NoError(err)
			sendAmt := ibctesting.DefaultCoinAmount
			escrowAddress := transfertypes.GetEscrowAddress(
				transfertypes.PortID,
				suite.pathAToB.EndpointA.ClientID,
			)
			packetData = transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				sendAmt.String(),
				suite.evmChainA.SenderAccount.GetAddress().String(),
				suite.chainB.SenderAccount.GetAddress().String(),
				"",
			)

			ack = channeltypes.NewResultAcknowledgement([]byte{1}).Acknowledgement()

			payload = channeltypesv2.NewPayload(
				transfertypes.PortID, transfertypes.PortID,
				transfertypes.V1, transfertypes.EncodingJSON,
				packetData.GetBytes(),
			)

			if tc.malleate != nil {
				tc.malleate()
			}

			// erc20 module is routed as top level middleware
			transferStack := suite.evmChainA.App.GetIBCKeeper().ChannelKeeperV2.Router.Route(ibctesting.TransferPort)
			if tc.onSendRequired {
				suite.NoError(transferStack.OnSendPacket(
					ctx,
					suite.pathAToB.EndpointA.ClientID,
					suite.pathAToB.EndpointB.ClientID,
					1,
					payload,
					suite.evmChainA.SenderAccount.GetAddress(),
				))
				// check that the escrowed coin is escrowed
				escrowedCoin := evmApp.BankKeeper.GetBalance(ctx, escrowAddress, bondDenom)
				suite.Require().Equal(escrowedCoin.Amount, sendAmt)
			}
			onAckPacket := func() error {
				return transferStack.OnAcknowledgementPacket(
					ctx,
					suite.pathAToB.EndpointA.ClientID,
					suite.pathAToB.EndpointB.ClientID,
					1,
					ack,
					payload,
					suite.evmChainA.SenderAccount.GetAddress(),
				)
			}

			err = onAckPacket()
			if tc.expError != "" {
				suite.Require().Error(err)
				suite.Require().ErrorContains(err, tc.expError)
			} else {
				suite.Require().NoError(err)
			}
			// check that the escrowed coins are un-escrowed
			if tc.onSendRequired && bytes.Equal(ack, channeltypesv2.ErrorAcknowledgement[:]) {
				escrowedCoins := evmApp.BankKeeper.GetAllBalances(ctx, escrowAddress)
				suite.Require().Equal(0, len(escrowedCoins))
			}
		})
	}
}

// TestOnAcknowledgementPacketNativeErc20 tests ack logic when the packet involves a native ERC20.
func (suite *MiddlewareV2TestSuite) TestOnAcknowledgementPacketNativeErc20() {
	var (
		payload channeltypesv2.Payload
		ack     []byte
	)

	testCases := []struct {
		name      string
		malleate  func()
		expError  string
		expRefund bool
	}{
		{
			name:      "pass",
			malleate:  nil,
			expError:  "",
			expRefund: false,
		},
		{
			name: "pass: refund escrowed token because ack err(UNIVERSAL_ERROR_ACKNOWLEDGEMENT)",
			malleate: func() {
				ack = channeltypesv2.ErrorAcknowledgement[:]
			},
			expError:  "",
			expRefund: true,
		},
		{
			name: "fail: malformed packet data",
			malleate: func() {
				payload.Value = []byte("malformed data")
			},
			expError:  "cannot unmarshal ICS20-V1 transfer packet data",
			expRefund: false,
		},
		{
			name: "fail: empty ack",
			malleate: func() {
				ack = []byte{}
			},
			expError:  "cannot unmarshal ICS-20 transfer packet acknowledgement",
			expRefund: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			nativeErc20 := SetupNativeErc20(suite.T(), suite.evmChainA)
			senderEthAddr := nativeErc20.Account
			sender := sdk.AccAddress(senderEthAddr.Bytes())
			sendAmt := math.NewIntFromBigInt(nativeErc20.InitialBal)

			evmCtx := suite.evmChainA.GetContext()
			evmApp := suite.evmChainA.App.(*evmd.EVMD)

			// MOCK erc20 native coin transfer from chainA to chainB
			// 1: Convert erc20 tokens to native erc20 coins for sending through IBC.
			_, err := evmApp.Erc20Keeper.ConvertERC20(
				evmCtx,
				types.NewMsgConvertERC20(
					sendAmt,
					sender,
					nativeErc20.ContractAddr,
					senderEthAddr,
				),
			)
			suite.Require().NoError(err)
			// 1-1: Check native erc20 token is converted to native erc20 coin on chainA.
			erc20BalAfterConvert := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
			suite.Require().Equal(
				new(big.Int).Sub(nativeErc20.InitialBal, sendAmt.BigInt()).String(),
				erc20BalAfterConvert.String(),
			)
			balAfterConvert := evmApp.BankKeeper.GetBalance(evmCtx, sender, nativeErc20.Denom)
			suite.Require().Equal(sendAmt.String(), balAfterConvert.Amount.String())

			path := suite.pathAToB
			escrowAddr := transfertypes.GetEscrowAddress(transfertypes.PortID, path.EndpointA.ClientID)
			// checkEscrow is a check function to ensure the native erc20 token is escrowed.
			checkEscrow := func() {
				erc20BalAfterIbcTransfer := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
				suite.Require().Equal(
					new(big.Int).Sub(nativeErc20.InitialBal, sendAmt.BigInt()).String(),
					erc20BalAfterIbcTransfer.String(),
				)
				escrowedBal := evmApp.BankKeeper.GetBalance(evmCtx, escrowAddr, nativeErc20.Denom)
				suite.Require().Equal(sendAmt.String(), escrowedBal.Amount.String())
			}

			// checkRefund is a check function to ensure refund is processed.
			checkRefund := func() {
				escrowedBal := evmApp.BankKeeper.GetBalance(evmCtx, escrowAddr, nativeErc20.Denom)
				suite.Require().True(escrowedBal.IsZero())

				// Check erc20 balance is same as initial balance after refund.
				erc20BalAfterIbcTransfer := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
				suite.Require().Equal(nativeErc20.InitialBal.String(), erc20BalAfterIbcTransfer.String())
			}

			// 2: Transfer erc20 native coin to chainB through IBC.
			chainBAcc := suite.chainB.SenderAccount.GetAddress()
			packetData := transfertypes.NewFungibleTokenPacketData(
				nativeErc20.Denom, sendAmt.String(),
				sender.String(), chainBAcc.String(),
				"",
			)
			payload = channeltypesv2.NewPayload(
				transfertypes.PortID, transfertypes.PortID,
				transfertypes.V1, transfertypes.EncodingJSON,
				packetData.GetBytes(),
			)
			timeoutTimestamp := uint64(suite.chainB.GetContext().BlockTime().Add(time.Hour).Unix()) //nolint:gosec // G115
			channelKeeperV2 := evmApp.GetIBCKeeper().ChannelKeeperV2
			_, err = channelKeeperV2.SendPacket(evmCtx, channeltypesv2.NewMsgSendPacket(
				path.EndpointA.ClientID,
				timeoutTimestamp,
				sender.String(),
				payload,
			))
			suite.Require().NoError(err)
			checkEscrow()

			ack = channeltypes.NewResultAcknowledgement([]byte{1}).Acknowledgement()
			if tc.malleate != nil {
				tc.malleate()
			}

			// 3: Mock sending back from chainB to chainA
			onAck := func() error {
				return channelKeeperV2.Router.Route(transfertypes.PortID).OnAcknowledgementPacket(
					evmCtx,
					path.EndpointA.ClientID,
					path.EndpointB.ClientID,
					0,
					ack,
					payload,
					sender,
				)
			}

			err = onAck()
			if tc.expError == "" {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.expError)
			}

			if tc.expRefund {
				checkRefund()
			} else {
				checkEscrow()
			}
		})
	}
}

func (suite *MiddlewareV2TestSuite) TestOnTimeoutPacket() {
	var (
		ctx        sdk.Context
		packetData transfertypes.FungibleTokenPacketData
		payload    channeltypesv2.Payload
	)

	testCases := []struct {
		name           string
		malleate       func()
		onSendRequired bool
		expError       string
	}{
		{
			name:           "pass",
			malleate:       nil,
			onSendRequired: true,
			expError:       "",
		},
		{
			name: "fail: malformed packet data",
			malleate: func() {
				payload.Value = []byte("malformed")
			},
			onSendRequired: false, // malformed packet data cannot be sent
			expError:       "cannot unmarshal ICS20-V1 transfer packet data",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx = suite.evmChainA.GetContext()
			evmApp := suite.evmChainA.App.(*evmd.EVMD)
			bondDenom, err := evmApp.StakingKeeper.BondDenom(ctx)
			suite.Require().NoError(err)
			packetData = transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				ibctesting.DefaultCoinAmount.String(),
				suite.evmChainA.SenderAccount.GetAddress().String(),
				suite.chainB.SenderAccount.GetAddress().String(),
				"",
			)

			payload = channeltypesv2.NewPayload(
				transfertypes.PortID, transfertypes.PortID,
				transfertypes.V1, transfertypes.EncodingJSON,
				packetData.GetBytes(),
			)

			if tc.malleate != nil {
				tc.malleate()
			}

			transferStack := suite.evmChainA.App.GetIBCKeeper().ChannelKeeperV2.Router.Route(ibctesting.TransferPort)
			if tc.onSendRequired {
				suite.NoError(transferStack.OnSendPacket(
					ctx,
					suite.pathAToB.EndpointA.ClientID,
					suite.pathAToB.EndpointB.ClientID,
					1,
					payload,
					suite.evmChainA.SenderAccount.GetAddress(),
				))
			}

			onTimeoutPacket := func() error {
				return transferStack.OnTimeoutPacket(
					ctx,
					suite.pathAToB.EndpointA.ClientID,
					suite.pathAToB.EndpointB.ClientID,
					1,
					payload,
					suite.evmChainA.SenderAccount.GetAddress(),
				)
			}

			err = onTimeoutPacket()
			if tc.expError != "" {
				suite.Require().Error(err)
				suite.Require().ErrorContains(err, tc.expError)
			} else {
				suite.Require().NoError(err)
				// check that the escrowed coins are un-escrowed
				escrowAddress := transfertypes.GetEscrowAddress(
					transfertypes.PortID,
					suite.pathAToB.EndpointA.ClientID,
				)
				escrowedCoins := evmApp.BankKeeper.GetAllBalances(ctx, escrowAddress)
				suite.Require().Equal(0, len(escrowedCoins))
			}
		})
	}
}

// TestOnTimeoutPacketNativeErc20 tests the OnTimeoutPacket logic when the packet involves a native ERC20.
func (suite *MiddlewareV2TestSuite) TestOnTimeoutPacketNativeErc20() {
	var payload channeltypesv2.Payload

	testCases := []struct {
		name      string
		malleate  func()
		expError  string
		expRefund bool
	}{
		{
			name:      "pass: refund escrowed native erc20 coin",
			malleate:  nil,
			expError:  "",
			expRefund: true,
		},
		{
			name: "fail: malformed packet data",
			malleate: func() {
				payload.Value = []byte("malformed data")
			},
			expError:  "cannot unmarshal ICS20-V1 transfer packet data",
			expRefund: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			nativeErc20 := SetupNativeErc20(suite.T(), suite.evmChainA)
			senderEthAddr := nativeErc20.Account
			sender := sdk.AccAddress(senderEthAddr.Bytes())
			sendAmt := math.NewIntFromBigInt(nativeErc20.InitialBal)

			evmCtx := suite.evmChainA.GetContext()
			evmApp := suite.evmChainA.App.(*evmd.EVMD)

			// MOCK erc20 native coin transfer from chainA to chainB
			// 1: Convert erc20 tokens to native erc20 coins for sending through IBC.
			_, err := evmApp.Erc20Keeper.ConvertERC20(
				evmCtx,
				types.NewMsgConvertERC20(
					sendAmt,
					sender,
					nativeErc20.ContractAddr,
					senderEthAddr,
				),
			)
			suite.Require().NoError(err)
			// 1-1: Check native erc20 token is converted to native erc20 coin on chainA.
			erc20BalAfterConvert := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
			suite.Require().Equal(
				new(big.Int).Sub(nativeErc20.InitialBal, sendAmt.BigInt()).String(),
				erc20BalAfterConvert.String(),
			)
			balAfterConvert := evmApp.BankKeeper.GetBalance(evmCtx, sender, nativeErc20.Denom)
			suite.Require().Equal(sendAmt.String(), balAfterConvert.Amount.String())

			path := suite.pathAToB
			escrowAddr := transfertypes.GetEscrowAddress(transfertypes.PortID, path.EndpointA.ClientID)
			// checkEscrow is a check function to ensure the native erc20 token is escrowed.
			checkEscrow := func() {
				erc20BalAfterIbcTransfer := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
				suite.Require().Equal(
					new(big.Int).Sub(nativeErc20.InitialBal, sendAmt.BigInt()).String(),
					erc20BalAfterIbcTransfer.String(),
				)
				escrowedBal := evmApp.BankKeeper.GetBalance(evmCtx, escrowAddr, nativeErc20.Denom)
				suite.Require().Equal(sendAmt.String(), escrowedBal.Amount.String())
			}

			// checkRefund is a check function to ensure refund is processed.
			checkRefund := func() {
				escrowedBal := evmApp.BankKeeper.GetBalance(evmCtx, escrowAddr, nativeErc20.Denom)
				suite.Require().True(escrowedBal.IsZero())

				// Check erc20 balance is same as initial balance after refund.
				erc20BalAfterIbcTransfer := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
				suite.Require().Equal(nativeErc20.InitialBal.String(), erc20BalAfterIbcTransfer.String())
			}

			// 2: Transfer erc20 native coin to chainB through IBC.
			chainBAcc := suite.chainB.SenderAccount.GetAddress()
			packetData := transfertypes.NewFungibleTokenPacketData(
				nativeErc20.Denom, sendAmt.String(),
				sender.String(), chainBAcc.String(),
				"",
			)
			payload = channeltypesv2.NewPayload(
				transfertypes.PortID, transfertypes.PortID,
				transfertypes.V1, transfertypes.EncodingJSON,
				packetData.GetBytes(),
			)
			timeoutTimestamp := uint64(suite.chainB.GetContext().BlockTime().Add(time.Hour).Unix()) //nolint:gosec // G115
			channelKeeperV2 := evmApp.GetIBCKeeper().ChannelKeeperV2
			_, err = channelKeeperV2.SendPacket(evmCtx, channeltypesv2.NewMsgSendPacket(
				path.EndpointA.ClientID,
				timeoutTimestamp,
				sender.String(),
				payload,
			))
			suite.Require().NoError(err)
			checkEscrow()

			if tc.malleate != nil {
				tc.malleate()
			}

			// 3: Trigger timeout on chainA
			onTimeout := func() error {
				return channelKeeperV2.Router.Route(transfertypes.PortID).OnTimeoutPacket(
					evmCtx,
					path.EndpointA.ClientID,
					path.EndpointB.ClientID,
					0,
					payload,
					sender,
				)
			}

			err = onTimeout()
			if tc.expError == "" {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.expError)
			}

			if tc.expRefund {
				checkRefund()
			} else {
				checkEscrow()
			}
		})
	}
}
