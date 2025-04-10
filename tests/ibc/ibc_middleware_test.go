package ibc

import (
	"errors"
	"math/big"
	"testing"

	testifysuite "github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/ibc"
	evmibctesting "github.com/cosmos/evm/ibc/testing"
	"github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/x/erc20"
	erc20Keeper "github.com/cosmos/evm/x/erc20/keeper"
	"github.com/cosmos/evm/x/erc20/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

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
func (s *MiddlewareTestSuite) SetupTest() {
	s.coordinator = evmibctesting.NewCoordinator(s.T(), 1, 2)
	s.evmChainA = s.coordinator.GetChain(ibctesting.GetChainID(1))
	s.chainB = s.coordinator.GetChain(ibctesting.GetChainID(2))

	// Setup path for A->B
	s.pathAToB = evmibctesting.NewPath(s.evmChainA, s.chainB)
	s.pathAToB.EndpointA.ChannelConfig.PortID = ibctesting.TransferPort
	s.pathAToB.EndpointB.ChannelConfig.PortID = ibctesting.TransferPort
	s.pathAToB.EndpointA.ChannelConfig.Version = transfertypes.V1
	s.pathAToB.EndpointB.ChannelConfig.Version = transfertypes.V1
	s.pathAToB.Setup()

	// Setup path for B->A
	s.pathBToA = evmibctesting.NewPath(s.chainB, s.evmChainA)
	s.pathBToA.EndpointA.ChannelConfig.PortID = ibctesting.TransferPort
	s.pathBToA.EndpointB.ChannelConfig.PortID = ibctesting.TransferPort
	s.pathBToA.EndpointA.ChannelConfig.Version = transfertypes.V1
	s.pathBToA.EndpointB.ChannelConfig.Version = transfertypes.V1
	s.pathBToA.Setup()
}

func TestMiddlewareTestSuite(t *testing.T) {
	testifysuite.Run(t, new(MiddlewareTestSuite))
}

// TestNewIBCMiddleware verifies the middleware instantiation logic.
func (s *MiddlewareTestSuite) TestNewIBCMiddleware() {
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
		s.Run(tc.name, func() {
			if tc.expError == nil {
				s.Require().NotPanics(
					tc.instantiateFn,
					"unexpected panic: NewIBCMiddleware",
				)
			} else {
				s.Require().PanicsWithError(
					tc.expError.Error(),
					tc.instantiateFn,
					"expected panic with error: ", tc.expError.Error(),
				)
			}
		})
	}
}

// TestOnRecvPacket checks the OnRecvPacket logic for ICS-20.
func (s *MiddlewareTestSuite) TestOnRecvPacket() {
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
		s.Run(tc.name, func() {
			s.SetupTest()

			ctxB := s.chainB.GetContext()
			bondDenom, err := s.chainB.GetSimApp().StakingKeeper.BondDenom(ctxB)
			s.Require().NoError(err)

			sendAmt := ibctesting.DefaultCoinAmount
			receiver := s.evmChainA.SenderAccount.GetAddress()

			packetData := transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				sendAmt.String(),
				s.chainB.SenderAccount.GetAddress().String(),
				receiver.String(),
				"",
			)
			path := s.pathBToA
			packet = channeltypes.Packet{
				Sequence:           1,
				SourcePort:         path.EndpointB.ChannelConfig.PortID,
				SourceChannel:      path.EndpointB.ChannelID,
				DestinationPort:    path.EndpointA.ChannelConfig.PortID,
				DestinationChannel: path.EndpointA.ChannelID,
				Data:               packetData.GetBytes(),
				TimeoutHeight:      s.evmChainA.GetTimeoutHeight(),
				TimeoutTimestamp:   0,
			}

			if tc.malleate != nil {
				tc.malleate()
			}

			transferStack, ok := s.evmChainA.App.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
			s.Require().True(ok)

			ctxA := s.evmChainA.GetContext()
			sourceChan := path.EndpointB.GetChannel()

			ack := transferStack.OnRecvPacket(
				ctxA,
				sourceChan.Version,
				packet,
				s.evmChainA.SenderAccount.GetAddress(),
			)

			if tc.expError == "" {
				s.Require().True(ack.Success())

				// Ensure ibc transfer from chainB to evmChainA is successful.
				data, ackErr := transfertypes.UnmarshalPacketData(packetData.GetBytes(), sourceChan.Version, "")
				s.Require().Nil(ackErr)

				voucherDenom := testutil.GetVoucherDenomFromPacketData(data, packet.GetDestPort(), packet.GetDestChannel())

				evmApp := s.evmChainA.App.(*evmd.EVMD)
				voucherCoin := evmApp.BankKeeper.GetBalance(ctxA, receiver, voucherDenom)
				s.Require().Equal(sendAmt.String(), voucherCoin.Amount.String())

				// Make sure token pair is registered
				tp, err := types.NewTokenPairSTRv2(voucherDenom)
				s.Require().NoError(err)
				tokenPair, found := evmApp.Erc20Keeper.GetTokenPair(ctxA, tp.GetID())
				s.Require().True(found)
				s.Require().Equal(voucherDenom, tokenPair.Denom)
			} else {
				s.Require().False(ack.Success())

				ackObj, ok := ack.(channeltypes.Acknowledgement)
				s.Require().True(ok)
				ackErr, ok := ackObj.Response.(*channeltypes.Acknowledgement_Error)
				s.Require().True(ok)
				s.Require().Contains(ackErr.Error, tc.expError)
			}
		})
	}
}

// TestOnRecvPacketNativeErc20 checks receiving a native ERC20 token.
func (s *MiddlewareTestSuite) TestOnRecvPacketNativeErc20() {
	s.SetupTest()
	nativeErc20 := SetupNativeErc20(s.T(), s.evmChainA)

	evmCtx := s.evmChainA.GetContext()
	evmApp := s.evmChainA.App.(*evmd.EVMD)

	// Scenario: Native ERC20 token transfer from evmChainA to chainB
	timeoutHeight := clienttypes.NewHeight(1, 110)
	path := s.pathAToB
	chainBAccount := s.chainB.SenderAccount.GetAddress()

	sendAmt := math.NewIntFromBigInt(nativeErc20.InitialBal)
	senderEthAddr := nativeErc20.Account
	sender := sdk.AccAddress(senderEthAddr.Bytes())

	msg := transfertypes.NewMsgTransfer(
		path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID,
		sdk.NewCoin(nativeErc20.Denom, sendAmt),
		sender.String(), chainBAccount.String(),
		timeoutHeight, 0, "",
	)
	_, err := s.evmChainA.SendMsgs(msg)
	s.Require().NoError(err) // message committed

	balAfterTransfer := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
	s.Require().Equal(
		new(big.Int).Sub(nativeErc20.InitialBal, sendAmt.BigInt()).String(),
		balAfterTransfer.String(),
	)

	// Check native erc20 token is escrowed on evmChainA for sending to chainB.
	escrowAddr := transfertypes.GetEscrowAddress(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
	escrowedBal := evmApp.BankKeeper.GetBalance(evmCtx, escrowAddr, nativeErc20.Denom)
	s.Require().Equal(sendAmt.String(), escrowedBal.Amount.String())

	// chainBNativeErc20Denom is the native erc20 token denom on chainB from evmChainA through IBC.
	chainBNativeErc20Denom := transfertypes.NewDenom(
		nativeErc20.Denom,
		transfertypes.NewHop(
			s.pathAToB.EndpointB.ChannelConfig.PortID,
			s.pathAToB.EndpointB.ChannelID,
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
		TimeoutHeight:      s.evmChainA.GetTimeoutHeight(),
		TimeoutTimestamp:   0,
	}

	transferStack, ok := s.evmChainA.App.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
	s.Require().True(ok)

	sourceChan := path.EndpointB.GetChannel()
	ack := transferStack.OnRecvPacket(
		evmCtx,
		sourceChan.Version,
		packet,
		s.evmChainA.SenderAccount.GetAddress(),
	)
	s.Require().True(ack.Success())

	// Check un-escrowed balance on evmChainA after receiving the packet.
	escrowedBal = evmApp.BankKeeper.GetBalance(evmCtx, escrowAddr, nativeErc20.Denom)
	s.Require().True(escrowedBal.IsZero(), "escrowed balance should be un-escrowed after receiving the packet")
	balAfterUnescrow := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
	s.Require().Equal(nativeErc20.InitialBal.String(), balAfterUnescrow.String())
}

func (s *MiddlewareTestSuite) TestOnAcknowledgementPacket() {
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
		s.Run(tc.name, func() {
			s.SetupTest()

			ctxA := s.evmChainA.GetContext()
			evmApp := s.evmChainA.App.(*evmd.EVMD)

			bondDenom, err := evmApp.StakingKeeper.BondDenom(ctxA)
			s.Require().NoError(err)

			sendAmt := ibctesting.DefaultCoinAmount
			sender := s.evmChainA.SenderAccount.GetAddress()
			receiver := s.chainB.SenderAccount.GetAddress()

			packetData := transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				sendAmt.String(),
				sender.String(),
				receiver.String(),
				"",
			)

			path := s.pathAToB
			packet = channeltypes.Packet{
				Sequence:           1,
				SourcePort:         path.EndpointA.ChannelConfig.PortID,
				SourceChannel:      path.EndpointA.ChannelID,
				DestinationPort:    path.EndpointB.ChannelConfig.PortID,
				DestinationChannel: path.EndpointB.ChannelID,
				Data:               packetData.GetBytes(),
				TimeoutHeight:      s.chainB.GetTimeoutHeight(),
				TimeoutTimestamp:   0,
			}

			ack = channeltypes.NewResultAcknowledgement([]byte{1}).Acknowledgement()
			if tc.malleate != nil {
				tc.malleate()
			}

			transferStack, ok := evmApp.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
			s.Require().True(ok)

			sourceChan := s.pathAToB.EndpointA.GetChannel()
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
				res, err := s.evmChainA.SendMsgs(msg)
				s.Require().NoError(err) // message committed

				packet, err := ibctesting.ParsePacketFromEvents(res.Events)
				s.Require().NoError(err)

				// relay the sent packet
				err = path.RelayPacket(packet)
				s.Require().NoError(err) // relay committed
			}

			err = onAck()
			if tc.expError == "" {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.expError)
			}
		})
	}
}

// TestOnAcknowledgementPacketNativeErc20 tests ack logic when the packet involves a native ERC20.
func (s *MiddlewareTestSuite) TestOnAcknowledgementPacketNativeErc20() {
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
		s.Run(tc.name, func() {
			s.SetupTest()
			nativeErc20 := SetupNativeErc20(s.T(), s.evmChainA)

			evmCtx := s.evmChainA.GetContext()
			evmApp := s.evmChainA.App.(*evmd.EVMD)

			timeoutHeight := clienttypes.NewHeight(1, 110)
			path := s.pathAToB
			chainBAccount := s.chainB.SenderAccount.GetAddress()

			sendAmt := math.NewIntFromBigInt(nativeErc20.InitialBal)
			senderEthAddr := nativeErc20.Account
			sender := sdk.AccAddress(senderEthAddr.Bytes())
			receiver := s.chainB.SenderAccount.GetAddress()

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
				s.Require().Equal(
					new(big.Int).Sub(nativeErc20.InitialBal, sendAmt.BigInt()).String(),
					erc20BalAfterIbcTransfer.String(),
				)
				escrowedBal := evmApp.BankKeeper.GetBalance(evmCtx, escrowAddr, nativeErc20.Denom)
				s.Require().Equal(sendAmt.String(), escrowedBal.Amount.String())
			}

			// checkRefund is a check function to ensure refund is processed.
			checkRefund := func() {
				escrowedBal := evmApp.BankKeeper.GetBalance(evmCtx, escrowAddr, nativeErc20.Denom)
				s.Require().True(escrowedBal.IsZero())

				// Check erc20 balance is same as initial balance after refund.
				erc20BalAfterIbcTransfer := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
				s.Require().Equal(nativeErc20.InitialBal.String(), erc20BalAfterIbcTransfer.String())
			}

			_, err := s.evmChainA.SendMsgs(msg)
			s.Require().NoError(err) // message committed
			checkEscrow()

			transferStack, ok := s.evmChainA.App.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
			s.Require().True(ok)

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
				TimeoutHeight:      s.chainB.GetTimeoutHeight(),
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
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.expError)
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
func (s *MiddlewareTestSuite) TestOnTimeoutPacket() {
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
		s.Run(tc.name, func() {
			s.SetupTest()

			ctxA := s.evmChainA.GetContext()
			evmApp := s.evmChainA.App.(*evmd.EVMD)
			bondDenom, err := evmApp.StakingKeeper.BondDenom(ctxA)
			s.Require().NoError(err)

			sendAmt := ibctesting.DefaultCoinAmount
			sender := s.evmChainA.SenderAccount.GetAddress()
			receiver := s.chainB.SenderAccount.GetAddress()

			packetData := transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				sendAmt.String(),
				sender.String(),
				receiver.String(),
				"",
			)

			path := s.pathAToB
			packet = channeltypes.Packet{
				Sequence:           1,
				SourcePort:         path.EndpointA.ChannelConfig.PortID,
				SourceChannel:      path.EndpointA.ChannelID,
				DestinationPort:    path.EndpointB.ChannelConfig.PortID,
				DestinationChannel: path.EndpointB.ChannelID,
				Data:               packetData.GetBytes(),
				TimeoutHeight:      s.chainB.GetTimeoutHeight(),
				TimeoutTimestamp:   0,
			}

			if tc.malleate != nil {
				tc.malleate()
			}

			transferStack, ok := evmApp.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
			s.Require().True(ok)

			sourceChan := s.pathAToB.EndpointA.GetChannel()
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

				res, err := s.evmChainA.SendMsgs(msg)
				s.Require().NoError(err) // message committed

				packet, err := ibctesting.ParsePacketFromEvents(res.Events)
				s.Require().NoError(err)

				err = path.RelayPacket(packet)
				s.Require().NoError(err) // relay committed
			}

			err = onTimeout()
			if tc.expError == "" {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.expError)
			}
		})
	}
}

// TestOnTimeoutPacketNativeErc20 tests the OnTimeoutPacket method for native ERC20 tokens.
func (s *MiddlewareTestSuite) TestOnTimeoutPacketNativeErc20() {
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
		s.Run(tc.name, func() {
			s.SetupTest()
			nativeErc20 := SetupNativeErc20(s.T(), s.evmChainA)

			evmCtx := s.evmChainA.GetContext()
			evmApp := s.evmChainA.App.(*evmd.EVMD)

			timeoutHeight := clienttypes.NewHeight(1, 110)
			path := s.pathAToB
			chainBAccount := s.chainB.SenderAccount.GetAddress()

			sendAmt := math.NewIntFromBigInt(nativeErc20.InitialBal)
			senderEthAddr := nativeErc20.Account
			sender := sdk.AccAddress(senderEthAddr.Bytes())
			receiver := s.chainB.SenderAccount.GetAddress()

			msg := transfertypes.NewMsgTransfer(
				path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID,
				sdk.NewCoin(nativeErc20.Denom, sendAmt), sender.String(), receiver.String(),
				timeoutHeight, 0, "",
			)

			escrowAddr := transfertypes.GetEscrowAddress(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
			// checkEscrow is a check function to ensure the native erc20 token is escrowed.
			checkEscrow := func() {
				erc20BalAfterIbcTransfer := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
				s.Require().Equal(
					new(big.Int).Sub(nativeErc20.InitialBal, sendAmt.BigInt()).String(),
					erc20BalAfterIbcTransfer.String(),
				)
				escrowedBal := evmApp.BankKeeper.GetBalance(evmCtx, escrowAddr, nativeErc20.Denom)
				s.Require().Equal(sendAmt.String(), escrowedBal.Amount.String())
			}

			// checkRefund is a check function to ensure refund is processed.
			checkRefund := func() {
				escrowedBal := evmApp.BankKeeper.GetBalance(evmCtx, escrowAddr, nativeErc20.Denom)
				s.Require().True(escrowedBal.IsZero())

				// Check erc20 balance is same as initial balance after refund.
				erc20BalAfterIbcTransfer := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
				s.Require().Equal(nativeErc20.InitialBal.String(), erc20BalAfterIbcTransfer.String())
			}
			_, err := s.evmChainA.SendMsgs(msg)
			s.Require().NoError(err) // message committed
			checkEscrow()

			transferStack, ok := s.evmChainA.App.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
			s.Require().True(ok)

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
				TimeoutHeight:      s.chainB.GetTimeoutHeight(),
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
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.expError)
			}

			if tc.expRefund {
				checkRefund()
			} else {
				checkEscrow()
			}
		})
	}
}
