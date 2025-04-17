package ibc

import (
	"errors"
	"testing"

	testifysuite "github.com/stretchr/testify/suite"

	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"
	ibcmockv2 "github.com/cosmos/ibc-go/v10/testing/mock/v2"

	"github.com/cosmos/evm/evmd"
	evmibctesting "github.com/cosmos/evm/ibc/testing"
	"github.com/cosmos/evm/testutil"
	erc20Keeper "github.com/cosmos/evm/x/erc20/keeper"
	"github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/erc20/v2"

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
	// pathBToA.EndpointA = endpoint on chainB
	// pathBToA.EndpointB = endpoint on evmChainA
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
			packetData = transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				ibctesting.DefaultCoinAmount.String(),
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
