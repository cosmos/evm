package ibc

import (
	"errors"
	"math/big"
	"testing"

	testifysuite "github.com/stretchr/testify/suite"

	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/ibc"
	evmibctesting "github.com/cosmos/evm/ibc/testing"
	"github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/x/erc20"
	erc20Keeper "github.com/cosmos/evm/x/erc20/keeper"
	"github.com/cosmos/evm/x/erc20/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MiddlewareTestSuite tests the IBC middleware for the ERC20 module.
type MiddlewareTestSuite struct {
	testifysuite.Suite

	coordinator *evmibctesting.Coordinator

	// testing chains used for convenience and readability
	evmChainA *evmibctesting.TestChain
	chainB    *evmibctesting.TestChain

	pathAToB *evmibctesting.Path
	pathBToA *evmibctesting.Path
}

// SetupTest initializes the coordinator and test chains before each test.
func (suite *MiddlewareTestSuite) SetupTest() {
	suite.coordinator = evmibctesting.NewCoordinator(suite.T(), 1, 2)
	suite.evmChainA = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
	suite.chainB = suite.coordinator.GetChain(ibctesting.GetChainID(2))

	// Setup path for A->B
	suite.pathAToB = evmibctesting.NewPath(suite.evmChainA, suite.chainB)
	suite.pathAToB.EndpointA.ChannelConfig.PortID = ibctesting.TransferPort
	suite.pathAToB.EndpointB.ChannelConfig.PortID = ibctesting.TransferPort
	suite.pathAToB.EndpointA.ChannelConfig.Version = transfertypes.V1
	suite.pathAToB.EndpointB.ChannelConfig.Version = transfertypes.V1
	suite.pathAToB.Setup()

	// Setup path for B->A
	suite.pathBToA = evmibctesting.NewPath(suite.chainB, suite.evmChainA)
	suite.pathBToA.EndpointA.ChannelConfig.PortID = ibctesting.TransferPort
	suite.pathBToA.EndpointB.ChannelConfig.PortID = ibctesting.TransferPort
	suite.pathBToA.EndpointA.ChannelConfig.Version = transfertypes.V1
	suite.pathBToA.EndpointB.ChannelConfig.Version = transfertypes.V1
	suite.pathBToA.Setup()
}

func TestMiddlewareTestSuite(t *testing.T) {
	testifysuite.Run(t, new(MiddlewareTestSuite))
}

// TestNewIBCMiddleware verifies the middleware instantiation logic.
func (suite *MiddlewareTestSuite) TestNewIBCMiddleware() {
	testCases := []struct {
		name          string
		instantiateFn func()
		expError      error
	}{
		{
			"success",
			func() {
				_ = erc20.NewIBCMiddleware(erc20Keeper.Keeper{}, ibc.Module{})
			},
			nil,
		},
		{
			"panics with nil underlying app",
			func() {
				_ = erc20.NewIBCMiddleware(erc20Keeper.Keeper{}, nil)
			},
			errors.New("underlying application cannot be nil"),
		},
		{
			"panics with nil erc20 keeper",
			func() {
				_ = erc20.NewIBCMiddleware(nil, ibc.Module{})
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

// TestOnRecvPacket checks the OnRecvPacket logic for ICS-20.
func (suite *MiddlewareTestSuite) TestOnRecvPacket() {
	var packet channeltypes.Packet

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
				packet.Data = []byte("malformed data")
			},
			expError: "handling packet",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			ctxB := suite.chainB.GetContext()
			bondDenom, err := suite.chainB.GetSimApp().StakingKeeper.BondDenom(ctxB)
			suite.Require().NoError(err)

			sendAmt := ibctesting.DefaultCoinAmount
			receiver := suite.evmChainA.SenderAccount.GetAddress()

			packetData := transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				sendAmt.String(),
				suite.chainB.SenderAccount.GetAddress().String(),
				receiver.String(),
				"",
			)
			path := suite.pathBToA
			packet = channeltypes.Packet{
				Sequence:           1,
				SourcePort:         path.EndpointB.ChannelConfig.PortID,
				SourceChannel:      path.EndpointB.ChannelID,
				DestinationPort:    path.EndpointA.ChannelConfig.PortID,
				DestinationChannel: path.EndpointA.ChannelID,
				Data:               packetData.GetBytes(),
				TimeoutHeight:      suite.evmChainA.GetTimeoutHeight(),
				TimeoutTimestamp:   0,
			}

			if tc.malleate != nil {
				tc.malleate()
			}

			transferStack, ok := suite.evmChainA.App.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
			suite.Require().True(ok)

			ctxA := suite.evmChainA.GetContext()
			sourceChan := path.EndpointB.GetChannel()

			ack := transferStack.OnRecvPacket(
				ctxA,
				sourceChan.Version,
				packet,
				suite.evmChainA.SenderAccount.GetAddress(),
			)

			if tc.expError == "" {
				suite.Require().True(ack.Success())

				// Ensure ibc transfer from chainB to evmChainA is successful.
				data, ackErr := transfertypes.UnmarshalPacketData(packetData.GetBytes(), sourceChan.Version, "")
				suite.Require().Nil(ackErr)

				voucherDenom := testutil.GetVoucherDenomFromPacketData(data, packet.GetDestPort(), packet.GetDestChannel())

				evmApp := suite.evmChainA.App.(*evmd.EVMD)
				voucherCoin := evmApp.BankKeeper.GetBalance(ctxA, receiver, voucherDenom)
				suite.Require().Equal(sendAmt.String(), voucherCoin.Amount.String())

				// Make sure token pair is registered
				singleTokenRepresentation, err := types.NewTokenPairSTRv2(voucherDenom)
				suite.Require().NoError(err)
				tokenPair, found := evmApp.Erc20Keeper.GetTokenPair(ctxA, singleTokenRepresentation.GetID())
				suite.Require().True(found)
				suite.Require().Equal(voucherDenom, tokenPair.Denom)
				// Make sure dynamic precompile is registered
				params := evmApp.Erc20Keeper.GetParams(ctxA)
				suite.Require().Contains(params.DynamicPrecompiles, tokenPair.Erc20Address)
			} else {
				suite.Require().False(ack.Success())

				ackObj, ok := ack.(channeltypes.Acknowledgement)
				suite.Require().True(ok)
				ackErr, ok := ackObj.Response.(*channeltypes.Acknowledgement_Error)
				suite.Require().True(ok)
				suite.Require().Contains(ackErr.Error, tc.expError)
			}
		})
	}
}

// TestOnRecvPacketNativeErc20 checks receiving a native ERC20 token.
func (suite *MiddlewareTestSuite) TestOnRecvPacketNativeErc20() {
	suite.SetupTest()
	nativeErc20 := SetupNativeErc20(suite.T(), suite.evmChainA)

	evmCtx := suite.evmChainA.GetContext()
	evmApp := suite.evmChainA.App.(*evmd.EVMD)

	// Scenario: Native ERC20 token transfer from evmChainA to chainB
	timeoutHeight := clienttypes.NewHeight(1, 110)
	path := suite.pathAToB
	chainBAccount := suite.chainB.SenderAccount.GetAddress()

	sendAmt := math.NewIntFromBigInt(nativeErc20.InitialBal)
	senderEthAddr := nativeErc20.Account
	sender := sdk.AccAddress(senderEthAddr.Bytes())

	msg := transfertypes.NewMsgTransfer(
		path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID,
		sdk.NewCoin(nativeErc20.Denom, sendAmt),
		sender.String(), chainBAccount.String(),
		timeoutHeight, 0, "",
	)
	_, err := suite.evmChainA.SendMsgs(msg)
	suite.Require().NoError(err) // message committed

	balAfterTransfer := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
	suite.Require().Equal(
		new(big.Int).Sub(nativeErc20.InitialBal, sendAmt.BigInt()).String(),
		balAfterTransfer.String(),
	)

	// Check native erc20 token is escrowed on evmChainA for sending to chainB.
	escrowAddr := transfertypes.GetEscrowAddress(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
	escrowedBal := evmApp.BankKeeper.GetBalance(evmCtx, escrowAddr, nativeErc20.Denom)
	suite.Require().Equal(sendAmt.String(), escrowedBal.Amount.String())

	// chainBNativeErc20Denom is the native erc20 token denom on chainB from evmChainA through IBC.
	chainBNativeErc20Denom := transfertypes.NewDenom(
		nativeErc20.Denom,
		transfertypes.NewHop(
			suite.pathAToB.EndpointB.ChannelConfig.PortID,
			suite.pathAToB.EndpointB.ChannelID,
		),
	)
	receiver := sender // the receiver is the sender on evmChainA
	// Mock the transfer of received native erc20 token by evmChainA to evmChainA.
	// Note that ChainB didn't receive the native erc20 token. We just assume that.
	packetData := transfertypes.NewFungibleTokenPacketData(
		chainBNativeErc20Denom.Path(),
		sendAmt.String(),
		chainBAccount.String(),
		receiver.String(),
		"",
	)
	packet := channeltypes.Packet{
		Sequence:           1,
		SourcePort:         path.EndpointB.ChannelConfig.PortID,
		SourceChannel:      path.EndpointB.ChannelID,
		DestinationPort:    path.EndpointA.ChannelConfig.PortID,
		DestinationChannel: path.EndpointA.ChannelID,
		Data:               packetData.GetBytes(),
		TimeoutHeight:      suite.evmChainA.GetTimeoutHeight(),
		TimeoutTimestamp:   0,
	}

	transferStack, ok := suite.evmChainA.App.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
	suite.Require().True(ok)

	sourceChan := path.EndpointB.GetChannel()
	ack := transferStack.OnRecvPacket(
		evmCtx,
		sourceChan.Version,
		packet,
		suite.evmChainA.SenderAccount.GetAddress(),
	)
	suite.Require().True(ack.Success())

	// Check un-escrowed balance on evmChainA after receiving the packet.
	escrowedBal = evmApp.BankKeeper.GetBalance(evmCtx, escrowAddr, nativeErc20.Denom)
	suite.Require().True(escrowedBal.IsZero(), "escrowed balance should be un-escrowed after receiving the packet")
	balAfterUnescrow := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
	suite.Require().Equal(nativeErc20.InitialBal.String(), balAfterUnescrow.String())
}

func (suite *MiddlewareTestSuite) TestOnAcknowledgementPacket() {
	var (
		packet channeltypes.Packet
		ack    []byte
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
			name: "pass: refund escrowed token",
			malleate: func() {
				ackErr := channeltypes.NewErrorAcknowledgement(errors.New("error"))
				ack = ackErr.Acknowledgement()
			},
			onSendRequired: true,
			expError:       "",
		},
		{
			name: "fail: malformed packet data",
			malleate: func() {
				packet.Data = []byte("malformed data")
			},
			onSendRequired: false,
			expError:       "cannot unmarshal ICS-20 transfer packet data",
		},
		{
			name: "fail: empty ack",
			malleate: func() {
				ack = []byte{}
			},
			onSendRequired: false,
			expError:       "cannot unmarshal ICS-20 transfer packet acknowledgement",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			ctxA := suite.evmChainA.GetContext()
			evmApp := suite.evmChainA.App.(*evmd.EVMD)

			bondDenom, err := evmApp.StakingKeeper.BondDenom(ctxA)
			suite.Require().NoError(err)

			sendAmt := ibctesting.DefaultCoinAmount
			sender := suite.evmChainA.SenderAccount.GetAddress()
			receiver := suite.chainB.SenderAccount.GetAddress()

			packetData := transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				sendAmt.String(),
				sender.String(),
				receiver.String(),
				"",
			)

			path := suite.pathAToB
			packet = channeltypes.Packet{
				Sequence:           1,
				SourcePort:         path.EndpointA.ChannelConfig.PortID,
				SourceChannel:      path.EndpointA.ChannelID,
				DestinationPort:    path.EndpointB.ChannelConfig.PortID,
				DestinationChannel: path.EndpointB.ChannelID,
				Data:               packetData.GetBytes(),
				TimeoutHeight:      suite.chainB.GetTimeoutHeight(),
				TimeoutTimestamp:   0,
			}

			ack = channeltypes.NewResultAcknowledgement([]byte{1}).Acknowledgement()
			if tc.malleate != nil {
				tc.malleate()
			}

			transferStack, ok := evmApp.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
			suite.Require().True(ok)

			sourceChan := suite.pathAToB.EndpointA.GetChannel()
			onAck := func() error {
				return transferStack.OnAcknowledgementPacket(
					ctxA,
					sourceChan.Version,
					packet,
					ack,
					receiver,
				)
			}
			if tc.onSendRequired {
				timeoutHeight := clienttypes.NewHeight(1, 110)
				msg := transfertypes.NewMsgTransfer(
					path.EndpointA.ChannelConfig.PortID,
					path.EndpointA.ChannelID,
					sdk.NewCoin(bondDenom, sendAmt),
					sender.String(),
					receiver.String(),
					timeoutHeight, 0, "",
				)
				res, err := suite.evmChainA.SendMsgs(msg)
				suite.Require().NoError(err) // message committed

				packet, err := ibctesting.ParsePacketFromEvents(res.Events)
				suite.Require().NoError(err)

				// relay the sent packet
				err = path.RelayPacket(packet)
				suite.Require().NoError(err) // relay committed
			}

			err = onAck()
			if tc.expError == "" {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.expError)
			}
		})
	}
}

// TestOnAcknowledgementPacketNativeErc20 tests ack logic when the packet involves a native ERC20.
func (suite *MiddlewareTestSuite) TestOnAcknowledgementPacketNativeErc20() {
	var (
		packet channeltypes.Packet
		ack    []byte
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
			name: "pass: refund escrowed token",
			malleate: func() {
				ackErr := channeltypes.NewErrorAcknowledgement(errors.New("error"))
				ack = ackErr.Acknowledgement()
			},
			expError:  "",
			expRefund: true,
		},
		{
			name: "fail: malformed packet data",
			malleate: func() {
				packet.Data = []byte("malformed data")
			},
			expError:  "cannot unmarshal ICS-20 transfer packet data",
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

			evmCtx := suite.evmChainA.GetContext()
			evmApp := suite.evmChainA.App.(*evmd.EVMD)

			timeoutHeight := clienttypes.NewHeight(1, 110)
			path := suite.pathAToB
			chainBAccount := suite.chainB.SenderAccount.GetAddress()

			sendAmt := math.NewIntFromBigInt(nativeErc20.InitialBal)
			senderEthAddr := nativeErc20.Account
			sender := sdk.AccAddress(senderEthAddr.Bytes())
			receiver := suite.chainB.SenderAccount.GetAddress()

			// Send the native erc20 token from evmChainA to chainB.
			msg := transfertypes.NewMsgTransfer(
				path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID,
				sdk.NewCoin(nativeErc20.Denom, sendAmt), sender.String(), receiver.String(),
				timeoutHeight, 0, "",
			)

			escrowAddr := transfertypes.GetEscrowAddress(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
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

			_, err := suite.evmChainA.SendMsgs(msg)
			suite.Require().NoError(err) // message committed
			checkEscrow()

			transferStack, ok := suite.evmChainA.App.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
			suite.Require().True(ok)

			packetData := transfertypes.NewFungibleTokenPacketData(
				nativeErc20.Denom,
				sendAmt.String(),
				sender.String(),
				chainBAccount.String(),
				"",
			)
			packet = channeltypes.Packet{
				Sequence:           1,
				SourcePort:         path.EndpointA.ChannelConfig.PortID,
				SourceChannel:      path.EndpointA.ChannelID,
				DestinationPort:    path.EndpointB.ChannelConfig.PortID,
				DestinationChannel: path.EndpointB.ChannelID,
				Data:               packetData.GetBytes(),
				TimeoutHeight:      suite.chainB.GetTimeoutHeight(),
				TimeoutTimestamp:   0,
			}

			ack = channeltypes.NewResultAcknowledgement([]byte{1}).Acknowledgement()
			if tc.malleate != nil {
				tc.malleate()
			}

			sourceChan := path.EndpointA.GetChannel()
			onAck := func() error {
				return transferStack.OnAcknowledgementPacket(
					evmCtx,
					sourceChan.Version,
					packet,
					ack,
					receiver,
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

// TestOnTimeoutPacket checks the timeout handling for ICS-20.
func (suite *MiddlewareTestSuite) TestOnTimeoutPacket() {
	var packet channeltypes.Packet

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
				packet.Data = []byte("malformed data")
			},
			onSendRequired: false,
			expError:       "cannot unmarshal ICS-20 transfer packet data",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			ctxA := suite.evmChainA.GetContext()
			evmApp := suite.evmChainA.App.(*evmd.EVMD)
			bondDenom, err := evmApp.StakingKeeper.BondDenom(ctxA)
			suite.Require().NoError(err)

			sendAmt := ibctesting.DefaultCoinAmount
			sender := suite.evmChainA.SenderAccount.GetAddress()
			receiver := suite.chainB.SenderAccount.GetAddress()

			packetData := transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				sendAmt.String(),
				sender.String(),
				receiver.String(),
				"",
			)

			path := suite.pathAToB
			packet = channeltypes.Packet{
				Sequence:           1,
				SourcePort:         path.EndpointA.ChannelConfig.PortID,
				SourceChannel:      path.EndpointA.ChannelID,
				DestinationPort:    path.EndpointB.ChannelConfig.PortID,
				DestinationChannel: path.EndpointB.ChannelID,
				Data:               packetData.GetBytes(),
				TimeoutHeight:      suite.chainB.GetTimeoutHeight(),
				TimeoutTimestamp:   0,
			}

			if tc.malleate != nil {
				tc.malleate()
			}

			transferStack, ok := evmApp.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
			suite.Require().True(ok)

			sourceChan := suite.pathAToB.EndpointA.GetChannel()
			onTimeout := func() error {
				return transferStack.OnTimeoutPacket(
					ctxA,
					sourceChan.Version,
					packet,
					sender,
				)
			}

			if tc.onSendRequired {
				timeoutHeight := clienttypes.NewHeight(1, 110)
				msg := transfertypes.NewMsgTransfer(
					path.EndpointA.ChannelConfig.PortID,
					path.EndpointA.ChannelID,
					sdk.NewCoin(bondDenom, sendAmt),
					sender.String(),
					receiver.String(),
					timeoutHeight, 0, "",
				)

				res, err := suite.evmChainA.SendMsgs(msg)
				suite.Require().NoError(err) // message committed

				packet, err := ibctesting.ParsePacketFromEvents(res.Events)
				suite.Require().NoError(err)

				err = path.RelayPacket(packet)
				suite.Require().NoError(err) // relay committed
			}

			err = onTimeout()
			if tc.expError == "" {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.expError)
			}
		})
	}
}

// TestOnTimeoutPacketNativeErc20 tests the OnTimeoutPacket method for native ERC20 tokens.
func (suite *MiddlewareTestSuite) TestOnTimeoutPacketNativeErc20() {
	var packet channeltypes.Packet

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
				packet.Data = []byte("malformed data")
			},
			expError:  "cannot unmarshal ICS-20 transfer packet data",
			expRefund: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			nativeErc20 := SetupNativeErc20(suite.T(), suite.evmChainA)

			evmCtx := suite.evmChainA.GetContext()
			evmApp := suite.evmChainA.App.(*evmd.EVMD)

			timeoutHeight := clienttypes.NewHeight(1, 110)
			path := suite.pathAToB
			chainBAccount := suite.chainB.SenderAccount.GetAddress()

			sendAmt := math.NewIntFromBigInt(nativeErc20.InitialBal)
			senderEthAddr := nativeErc20.Account
			sender := sdk.AccAddress(senderEthAddr.Bytes())
			receiver := suite.chainB.SenderAccount.GetAddress()

			msg := transfertypes.NewMsgTransfer(
				path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID,
				sdk.NewCoin(nativeErc20.Denom, sendAmt), sender.String(), receiver.String(),
				timeoutHeight, 0, "",
			)

			escrowAddr := transfertypes.GetEscrowAddress(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
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
			_, err := suite.evmChainA.SendMsgs(msg)
			suite.Require().NoError(err) // message committed
			checkEscrow()

			transferStack, ok := suite.evmChainA.App.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
			suite.Require().True(ok)

			packetData := transfertypes.NewFungibleTokenPacketData(
				nativeErc20.Denom,
				sendAmt.String(),
				sender.String(),
				chainBAccount.String(),
				"",
			)
			packet = channeltypes.Packet{
				Sequence:           1,
				SourcePort:         path.EndpointA.ChannelConfig.PortID,
				SourceChannel:      path.EndpointA.ChannelID,
				DestinationPort:    path.EndpointB.ChannelConfig.PortID,
				DestinationChannel: path.EndpointB.ChannelID,
				Data:               packetData.GetBytes(),
				TimeoutHeight:      suite.chainB.GetTimeoutHeight(),
				TimeoutTimestamp:   0,
			}

			if tc.malleate != nil {
				tc.malleate()
			}

			sourceChan := path.EndpointA.GetChannel()
			err = transferStack.OnTimeoutPacket(
				evmCtx,
				sourceChan.Version,
				packet,
				receiver,
			)

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
