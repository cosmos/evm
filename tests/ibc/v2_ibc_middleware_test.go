package ibc

import (
	"errors"
	"testing"

	testifysuite "github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd"
	evmibctesting "github.com/cosmos/evm/ibc/testing"
	"github.com/cosmos/evm/testutil"
	erc20Keeper "github.com/cosmos/evm/x/erc20/keeper"
	"github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/erc20/v2"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"
	ibcmockv2 "github.com/cosmos/ibc-go/v10/testing/mock/v2"

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

func (s *MiddlewareV2TestSuite) SetupTest() {
	s.coordinator = evmibctesting.NewCoordinator(s.T(), 1, 1)
	s.evmChainA = s.coordinator.GetChain(evmibctesting.GetChainID(1))
	s.chainB = s.coordinator.GetChain(evmibctesting.GetChainID(2))

	// setup between evmChainA and chainB
	// pathAToB.EndpointA = endpoint on evmChainA
	// pathAToB.EndpointB = endpoint on chainB
	s.pathAToB = evmibctesting.NewPath(s.evmChainA, s.chainB)
	// setup between chainB and evmChainA
	// pathBToA.EndpointA = endpoint on chainB
	// pathBToA.EndpointB = endpoint on evmChainA
	s.pathBToA = evmibctesting.NewPath(s.chainB, s.evmChainA)

	// setup IBC v2 paths between the chains
	s.pathAToB.SetupV2()
	s.pathBToA.SetupV2()
}

func TestMiddlewareV2TestSuite(t *testing.T) {
	testifysuite.Run(t, new(MiddlewareV2TestSuite))
}

func (s *MiddlewareV2TestSuite) TestNewIBCMiddleware() {
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

func (s *MiddlewareV2TestSuite) TestOnSendPacket() {
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
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.evmChainA.GetContext()
			evmApp := s.evmChainA.App.(*evmd.EVMD)
			bondDenom, err := evmApp.StakingKeeper.BondDenom(ctx)
			s.Require().NoError(err)
			packetData = transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				ibctesting.DefaultCoinAmount.String(),
				s.evmChainA.SenderAccount.GetAddress().String(),
				s.chainB.SenderAccount.GetAddress().String(),
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
					s.pathAToB.EndpointA.ClientID,
					s.pathAToB.EndpointB.ClientID,
					1,
					payload,
					s.evmChainA.SenderAccount.GetAddress(),
				)
			}

			err = onSendPacket()
			if tc.expError != "" {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.expError)
			} else {
				s.Require().NoError(err)
				// check that the escrowed coins are in the escrow account
				escrowAddress := transfertypes.GetEscrowAddress(
					transfertypes.PortID,
					s.pathAToB.EndpointA.ClientID,
				)
				escrowedCoins := evmApp.BankKeeper.GetAllBalances(ctx, escrowAddress)
				s.Require().Equal(1, len(escrowedCoins))
				s.Require().Equal(ibctesting.DefaultCoinAmount.String(), escrowedCoins[0].Amount.String())
				s.Require().Equal(bondDenom, escrowedCoins[0].Denom)
			}
		})
	}
}

func (s *MiddlewareV2TestSuite) TestOnRecvPacket() {
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
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.chainB.GetContext()
			bondDenom, err := s.chainB.GetSimApp().StakingKeeper.BondDenom(ctx)
			s.Require().NoError(err)
			receiver := s.evmChainA.SenderAccount.GetAddress()
			sendAmt := ibctesting.DefaultCoinAmount
			packetData = transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				sendAmt.String(),
				s.chainB.SenderAccount.GetAddress().String(),
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

			evmApp := s.evmChainA.App.(*evmd.EVMD)
			// erc20 module is routed as top level middleware
			transferStack := evmApp.GetIBCKeeper().ChannelKeeperV2.Router.Route(ibctesting.TransferPort)
			sourceClient := s.pathBToA.EndpointB.ClientID
			onRecvPacket := func() channeltypesv2.RecvPacketResult {
				ctx = s.evmChainA.GetContext()
				return transferStack.OnRecvPacket(
					ctx,
					sourceClient,
					s.pathBToA.EndpointA.ClientID,
					1,
					payload,
					receiver,
				)
			}

			recvResult := onRecvPacket()
			s.Require().Equal(tc.expResult, recvResult.Status)
			if recvResult.Status == channeltypesv2.PacketStatus_Success {
				// make sure voucher coins are sent to the receiver
				data, ackErr := transfertypes.UnmarshalPacketData(packetData.GetBytes(), transfertypes.V1, "")
				s.Require().Nil(ackErr)
				voucherDenom := testutil.GetVoucherDenomFromPacketData(data, payload.GetSourcePort(), sourceClient)
				voucherCoin := evmApp.BankKeeper.GetBalance(ctx, receiver, voucherDenom)
				s.Require().Equal(sendAmt.String(), voucherCoin.Amount.String())
				// make sure token pair is registered
				tp, err := types.NewTokenPairSTRv2(voucherDenom)
				s.Require().NoError(err)
				tokenPair, found := evmApp.Erc20Keeper.GetTokenPair(ctx, tp.GetID())
				s.Require().True(found)
				s.Require().Equal(voucherDenom, tokenPair.Denom)
			}
		})
	}
}

func (s *MiddlewareV2TestSuite) TestOnAcknowledgementPacket() {
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
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.evmChainA.GetContext()
			evmApp := s.evmChainA.App.(*evmd.EVMD)
			bondDenom, err := evmApp.StakingKeeper.BondDenom(ctx)
			s.Require().NoError(err)
			packetData = transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				ibctesting.DefaultCoinAmount.String(),
				s.evmChainA.SenderAccount.GetAddress().String(),
				s.chainB.SenderAccount.GetAddress().String(),
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
			transferStack := s.evmChainA.App.GetIBCKeeper().ChannelKeeperV2.Router.Route(ibctesting.TransferPort)
			if tc.onSendRequired {
				s.NoError(transferStack.OnSendPacket(
					ctx,
					s.pathAToB.EndpointA.ClientID,
					s.pathAToB.EndpointB.ClientID,
					1,
					payload,
					s.evmChainA.SenderAccount.GetAddress(),
				))
			}
			onAckPacket := func() error {
				return transferStack.OnAcknowledgementPacket(
					ctx,
					s.pathAToB.EndpointA.ClientID,
					s.pathAToB.EndpointB.ClientID,
					1,
					ack,
					payload,
					s.evmChainA.SenderAccount.GetAddress(),
				)
			}

			err = onAckPacket()
			if tc.expError != "" {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.expError)
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

func (s *MiddlewareV2TestSuite) TestOnTimeoutPacket() {
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
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.evmChainA.GetContext()
			evmApp := s.evmChainA.App.(*evmd.EVMD)
			bondDenom, err := evmApp.StakingKeeper.BondDenom(ctx)
			s.Require().NoError(err)
			packetData = transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				ibctesting.DefaultCoinAmount.String(),
				s.evmChainA.SenderAccount.GetAddress().String(),
				s.chainB.SenderAccount.GetAddress().String(),
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

			transferStack := s.evmChainA.App.GetIBCKeeper().ChannelKeeperV2.Router.Route(ibctesting.TransferPort)
			if tc.onSendRequired {
				s.NoError(transferStack.OnSendPacket(
					ctx,
					s.pathAToB.EndpointA.ClientID,
					s.pathAToB.EndpointB.ClientID,
					1,
					payload,
					s.evmChainA.SenderAccount.GetAddress(),
				))
			}

			onTimeoutPacket := func() error {
				return transferStack.OnTimeoutPacket(
					ctx,
					s.pathAToB.EndpointA.ClientID,
					s.pathAToB.EndpointB.ClientID,
					1,
					payload,
					s.evmChainA.SenderAccount.GetAddress(),
				)
			}

			err = onTimeoutPacket()
			if tc.expError != "" {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.expError)
			} else {
				s.Require().NoError(err)
				// check that the escrowed coins are un-escrowed
				escrowAddress := transfertypes.GetEscrowAddress(
					transfertypes.PortID,
					s.pathAToB.EndpointA.ClientID,
				)
				escrowedCoins := evmApp.BankKeeper.GetAllBalances(ctx, escrowAddress)
				s.Require().Equal(0, len(escrowedCoins))
			}
		})
	}
}
