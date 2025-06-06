package ibc

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	testifysuite "github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/ibc"
	evmibctesting "github.com/cosmos/evm/ibc/testing"
	"github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/testutil/integration/os/factory"
	"github.com/cosmos/evm/x/erc20"
	erc20Keeper "github.com/cosmos/evm/x/erc20/keeper"
	"github.com/cosmos/evm/x/erc20/types"
	testutil2 "github.com/cosmos/evm/x/ibc/callbacks/testutil"
	types2 "github.com/cosmos/evm/x/ibc/callbacks/types"
	ibctransfer "github.com/cosmos/ibc-go/v10/modules/apps/transfer"
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

	path *evmibctesting.Path
}

// SetupTest initializes the coordinator and test chains before each test.
func (suite *MiddlewareTestSuite) SetupTest() {
	suite.coordinator = evmibctesting.NewCoordinator(suite.T(), 1, 1)
	suite.evmChainA = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
	suite.chainB = suite.coordinator.GetChain(evmibctesting.GetChainID(2))

	// Setup path
	suite.path = evmibctesting.NewPath(suite.evmChainA, suite.chainB)
	suite.path.EndpointA.ChannelConfig.PortID = ibctesting.TransferPort
	suite.path.EndpointB.ChannelConfig.PortID = ibctesting.TransferPort
	suite.path.EndpointA.ChannelConfig.Version = transfertypes.V1
	suite.path.EndpointB.ChannelConfig.Version = transfertypes.V1
	suite.path.Setup()

	// ensure the channel is found to verify proper setup
	_, found := suite.evmChainA.App.GetIBCKeeper().ChannelKeeper.GetChannel(suite.evmChainA.GetContext(), suite.path.EndpointA.ChannelConfig.PortID, suite.path.EndpointA.ChannelID)
	suite.Require().True(found)
}

func TestMiddlewareTestSuite(t *testing.T) {
	testifysuite.Run(t, new(MiddlewareTestSuite))
}

// TestOnRecvPacket checks the OnRecvPacket logic for ICS-20.
func (suite *MiddlewareTestSuite) TestOnRecvPacketWithCallback() {
	var packet channeltypes.Packet

	testCases := []struct {
		name     string
		malleate func()
		memo     func() string
		expError string
	}{
		// todo: more cases
		{
			name:     "pass - callback to function",
			malleate: nil,
			memo: func() string { // todo: actually malleate the memo
				return ""
			},
			expError: "",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			path := suite.path

			ctxB := suite.chainB.GetContext()
			evmCtx := suite.evmChainA.GetContext()
			bondDenom, err := suite.chainB.GetSimApp().StakingKeeper.BondDenom(ctxB)
			suite.Require().NoError(err)

			// generate the isolated address for the sender
			sendAmt := ibctesting.DefaultCoinAmount
			isolatedAddr := types2.GenerateIsolatedAddress(path.EndpointA.ChannelID, suite.chainB.SenderAccount.GetAddress().String())

			// get callback tester contract and deploy it
			contractData, err := testutil2.LoadCounterWithCallbacksContract()
			suite.Require().NoError(err)

			deploymentData := factory.ContractDeploymentData{
				Contract:        contractData,
				ConstructorArgs: nil,
			}

			contractAddr, err := DeployContract(suite.T(), suite.evmChainA, deploymentData)
			suite.Require().NoError(err)

			// generate packet to execute the tester contract using callbacks
			packetData := transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				sendAmt.String(),
				suite.chainB.SenderAccount.GetAddress().String(),
				isolatedAddr.String(),
				tc.memo(),
			)

			_ = path.EndpointA.GetChannel()
			sourceChan := path.EndpointB.GetChannel()

			data, ackErr := transfertypes.UnmarshalPacketData(packetData.GetBytes(), sourceChan.Version, "")
			suite.Require().Nil(ackErr)

			voucherDenom := testutil.GetVoucherDenomFromPacketData(data, path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)

			// ensure token pair is registered
			singleTokenRepresentation, err := types.NewTokenPairSTRv2(voucherDenom)
			erc20Contract := singleTokenRepresentation.GetERC20Contract()
			suite.Require().NoError(err)

			amountInt, ok := math.NewIntFromString(packetData.Amount)
			suite.Require().True(ok)

			packedBytes, err := contractData.ABI.Pack("add", erc20Contract, amountInt.BigInt())
			suite.Require().NoError(err)

			destCallback := fmt.Sprintf(`{
			   "dest_callback": {
				  "address": "%s",
				  "gas_limit": "%d",
				  "calldata": "%x"
				}
        	}`, contractAddr, 1_000_000, packedBytes)

			packetData.Memo = destCallback

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

			//  transfer stack
			transferStack, ok := suite.evmChainA.App.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
			suite.Require().True(ok)

			_, found := suite.evmChainA.App.GetIBCKeeper().ChannelKeeper.GetChannel(evmCtx, path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
			suite.Require().True(found)

			ack := transferStack.OnRecvPacket(
				evmCtx,
				sourceChan.Version,
				packet,
				suite.evmChainA.SenderAccount.GetAddress(),
			)

			if tc.expError == "" {
				suite.Require().True(ack.Success())

				// Ensure ibc transfer from chainB to evmChainA is successful.
				_, ackErr = transfertypes.UnmarshalPacketData(packetData.GetBytes(), sourceChan.Version, "")
				suite.Require().Nil(ackErr)

				// check that the tester contract received the full token amount
				evmApp := suite.evmChainA.App.(*evmd.EVMD)
				balAfterUnescrow := evmApp.Erc20Keeper.BalanceOf(evmCtx, contracts.ERC20MinterBurnerDecimalsContract.ABI, erc20Contract, contractAddr)
				contractAccAddr, err := sdk.AccAddressFromHexUnsafe(contractAddr.String()[2:])
				suite.Require().NoError(err)
				voucherCoin := evmApp.BankKeeper.GetBalance(evmCtx, contractAccAddr, voucherDenom)
				suite.Require().Equal(sendAmt.String(), balAfterUnescrow.String())
				suite.Require().Equal(sendAmt.String(), voucherCoin.Amount.String())

				suite.Require().NoError(err)
				tokenPair, found := evmApp.Erc20Keeper.GetTokenPair(evmCtx, singleTokenRepresentation.GetID())
				suite.Require().True(found)
				suite.Require().Equal(voucherDenom, tokenPair.Denom)
				// Make sure dynamic precompile is registered
				params := evmApp.Erc20Keeper.GetParams(evmCtx)
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
				_ = erc20.NewIBCMiddleware(erc20Keeper.Keeper{}, ibctransfer.IBCModule{})
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
			path := suite.path
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
	path := suite.path
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
			suite.path.EndpointB.ChannelConfig.PortID,
			suite.path.EndpointB.ChannelID,
		),
	)
	// receiver := sender // the receiver is the sender on evmChainA
	// Mock the transfer of received native erc20 token by evmChainA to evmChainA.
	// Note that ChainB didn't receive the native erc20 token. We just assume that.
	packetData := transfertypes.NewFungibleTokenPacketData(
		chainBNativeErc20Denom.Path(),
		sendAmt.String(),
		chainBAccount.String(),
		types2.GenerateIsolatedAddress(path.EndpointA.ChannelID, suite.chainB.SenderAccount.GetAddress().String()).String(),
		"",
	)

	// get callback tester contract and deploy it
	contractData, err := testutil2.LoadCounterWithCallbacksContract()
	suite.Require().NoError(err)

	deploymentData := factory.ContractDeploymentData{
		Contract:        contractData,
		ConstructorArgs: nil,
	}

	contractAddr, err := DeployContract(suite.T(), suite.evmChainA, deploymentData)
	if err != nil {
		return
	}

	packedBytes, err := contractData.ABI.Pack("add", nativeErc20.ContractAddr, sendAmt.BigInt())
	suite.Require().NoError(err)

	destCallback := fmt.Sprintf(`{
			   "dest_callback": {
				  "address": "%s",
				  "gas_limit": "%d",
				  "calldata": "%x"
				}
        	}`, contractAddr, 1_000_000, packedBytes)

	packetData.Memo = destCallback

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

	suite.evmChainA.NextBlock()

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
	balAfterUnescrow := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, contractAddr)
	suite.Require().Equal(nativeErc20.InitialBal.String(), balAfterUnescrow.String())
	bankBalAfterUnescrow := evmApp.BankKeeper.GetBalance(evmCtx, sender, nativeErc20.Denom)
	suite.Require().True(bankBalAfterUnescrow.IsZero(), "no duplicate state in the bank balance")
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
			balBeforeTransfer := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)

			packetData := transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				sendAmt.String(),
				sender.String(),
				receiver.String(),
				"",
			)

			path := suite.path
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

			sourceChan := suite.path.EndpointA.GetChannel()
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

				packet, err := ibctesting.ParseV1PacketFromEvents(res.Events)
				suite.Require().NoError(err)

				// relay the sent packet
				err = path.RelayPacket(packet)
				suite.Require().NoError(err) // relay committed

				// ensure the ibc token is escrowed.
				balAfterTransfer := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)
				suite.Require().Equal(
					balBeforeTransfer.Amount.Sub(sendAmt).String(),
					balAfterTransfer.Amount.String(),
				)
				escrowAddr := transfertypes.GetEscrowAddress(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
				escrowedBal := evmApp.BankKeeper.GetBalance(ctxA, escrowAddr, bondDenom)
				suite.Require().Equal(sendAmt.String(), escrowedBal.Amount.String())
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
			path := suite.path
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
			balBeforeTransfer := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)

			packetData := transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				sendAmt.String(),
				sender.String(),
				receiver.String(),
				"",
			)

			path := suite.path
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

			sourceChan := suite.path.EndpointA.GetChannel()
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

				packet, err := ibctesting.ParseV1PacketFromEvents(res.Events)
				suite.Require().NoError(err)

				err = path.RelayPacket(packet)
				suite.Require().NoError(err) // relay committed

			}
			err = onTimeout()
			// ensure that the escrowed coins were refunded on timeout.
			balAfterTransfer := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)
			escrowAddr := transfertypes.GetEscrowAddress(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
			escrowedBal := evmApp.BankKeeper.GetBalance(ctxA, escrowAddr, bondDenom)
			suite.Require().Equal(
				balBeforeTransfer.Amount.String(),
				balAfterTransfer.Amount.String(),
			)
			suite.Require().Equal(escrowedBal.Amount.String(), math.ZeroInt().String())

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
			path := suite.path
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
