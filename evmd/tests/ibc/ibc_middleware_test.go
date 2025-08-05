package ibc

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	testifysuite "github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/ibc"
	"github.com/cosmos/evm/testutil"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	"github.com/cosmos/evm/x/erc20"
	erc20Keeper "github.com/cosmos/evm/x/erc20/keeper"
	"github.com/cosmos/evm/x/erc20/types"
	testutil2 "github.com/cosmos/evm/x/ibc/callbacks/testutil"
	types2 "github.com/cosmos/evm/x/ibc/callbacks/types"
	types3 "github.com/cosmos/evm/x/vm/types"
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
	suite.coordinator = evmibctesting.NewCoordinator(suite.T(), 1, 1, integration.SetupEvmd)
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

// TestOnRecvPacketWithCallback checks the OnRecvPacket logic for ICS-20 with comprehensive callback scenarios.
func (suite *MiddlewareTestSuite) TestOnRecvPacketWithCallback() {
	var packet channeltypes.Packet

	var contractData types3.CompiledContract
	var contractAddr common.Address
	var voucherDenom string
	var path *evmibctesting.Path
	var data transfertypes.InternalTransferRepresentation

	testCases := []struct {
		name     string
		malleate func()
		memo     func() string
		expError string
	}{
		// SUCCESS CASES
		{
			name:     "success - callback to add function with valid parameters",
			malleate: nil,
			memo: func() string {
				// Only the 'add' function properly transfers tokens
				amountInt, _ := math.NewIntFromString(ibctesting.DefaultCoinAmount.String())
				voucherDenom := testutil.GetVoucherDenomFromPacketData(data, path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
				singleTokenRepresentation, _ := types.NewTokenPairSTRv2(voucherDenom)
				erc20Contract := singleTokenRepresentation.GetERC20Contract()
				packedBytes, _ := contractData.ABI.Pack("add", erc20Contract, amountInt.BigInt())

				return fmt.Sprintf(`{
					"dest_callback": {
						"address": "%s",
						"gas_limit": "%d",
						"calldata": "%x"
					}
				}`, contractAddr, 1_000_000, packedBytes)
			},
			expError: "",
		},
		{
			name:     "success - callback with maximum gas limit",
			malleate: nil,
			memo: func() string {
				amountInt, _ := math.NewIntFromString(ibctesting.DefaultCoinAmount.String())
				voucherDenom := testutil.GetVoucherDenomFromPacketData(data, path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
				singleTokenRepresentation, _ := types.NewTokenPairSTRv2(voucherDenom)
				erc20Contract := singleTokenRepresentation.GetERC20Contract()
				packedBytes, _ := contractData.ABI.Pack("add", erc20Contract, amountInt.BigInt())

				return fmt.Sprintf(`{
					"dest_callback": {
						"address": "%s",
						"gas_limit": "%d",
						"calldata": "%x"
					}
				}`, contractAddr, 10_000_000, packedBytes)
			},
			expError: "",
		},

		// FAILURE CASES - Invalid Contract
		{
			name:     "failure - callback to non-existent contract",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"dest_callback": {
						"address": "0x1234567890123456789012345678901234567890",
						"gas_limit": "%d",
						"calldata": ""
					}
				}`, 1_000_000)
			},
			expError: "ABCI code: 4",
		},
		{
			name:     "failure - callback to empty address",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"dest_callback": {
						"address": "0x0000000000000000000000000000000000000000",
						"gas_limit": "%d",
						"calldata": ""
					}
				}`, 1_000_000)
			},
			expError: "ABCI code: 4",
		},

		// FAILURE CASES - Invalid Functions
		{
			name:     "failure - calling non-existent function",
			malleate: nil,
			memo: func() string {
				// Invalid function selector
				packedBytes := []byte{0xff, 0xff, 0xff, 0xff}

				return fmt.Sprintf(`{
					"dest_callback": {
						"address": "%s",
						"gas_limit": "%d",
						"calldata": "%x"
					}
				}`, contractAddr, 1_000_000, packedBytes)
			},
			expError: "ABCI code: 8",
		},
		{
			name:     "failure - calling getCounter function (doesn't transfer tokens)",
			malleate: nil,
			memo: func() string {
				packedBytes, _ := contractData.ABI.Pack("getCounter")

				return fmt.Sprintf(`{
					"dest_callback": {
						"address": "%s",
						"gas_limit": "%d",
						"calldata": "%x"
					}
				}`, contractAddr, 1_000_000, packedBytes)
			},
			expError: "ABCI code: 12",
		},
		{
			name:     "failure - calling resetCounter function (doesn't transfer tokens)",
			malleate: nil,
			memo: func() string {
				packedBytes, _ := contractData.ABI.Pack("resetCounter")

				return fmt.Sprintf(`{
					"dest_callback": {
						"address": "%s",
						"gas_limit": "%d",
						"calldata": "%x"
					}
				}`, contractAddr, 1_000_000, packedBytes)
			},
			expError: "ABCI code: 12",
		},
		{
			name:     "failure - calling add function with wrong parameters",
			malleate: nil,
			memo: func() string {
				// Invalid calldata for add function
				packedBytes := []byte{0x12, 0x34, 0x56, 0x78}

				return fmt.Sprintf(`{
					"dest_callback": {
						"address": "%s",
						"gas_limit": "%d",
						"calldata": "%x"
					}
				}`, contractAddr, 1_000_000, packedBytes)
			},
			expError: "ABCI code: 8",
		},
		{
			name:     "failure - calling add function with zero amount",
			malleate: nil,
			memo: func() string {
				voucherDenom := testutil.GetVoucherDenomFromPacketData(data, path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
				singleTokenRepresentation, _ := types.NewTokenPairSTRv2(voucherDenom)
				erc20Contract := singleTokenRepresentation.GetERC20Contract()
				packedBytes, _ := contractData.ABI.Pack("add", erc20Contract, big.NewInt(0))

				return fmt.Sprintf(`{
					"dest_callback": {
						"address": "%s",
						"gas_limit": "%d",
						"calldata": "%x"
					}
				}`, contractAddr, 1_000_000, packedBytes)
			},
			expError: "ABCI code: 8",
		},

		// FAILURE CASES - Gas Issues
		{
			name:     "failure - insufficient gas limit",
			malleate: nil,
			memo: func() string {
				amountInt, _ := math.NewIntFromString(ibctesting.DefaultCoinAmount.String())
				voucherDenom := testutil.GetVoucherDenomFromPacketData(data, path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
				singleTokenRepresentation, _ := types.NewTokenPairSTRv2(voucherDenom)
				erc20Contract := singleTokenRepresentation.GetERC20Contract()
				packedBytes, _ := contractData.ABI.Pack("add", erc20Contract, amountInt.BigInt())

				return fmt.Sprintf(`{
					"dest_callback": {
						"address": "%s",
						"gas_limit": "%d",
						"calldata": "%x"
					}
				}`, contractAddr, 1000, packedBytes) // Very low gas
			},
			expError: "ABCI code: 6",
		},
		{
			name:     "failure - zero gas limit",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"dest_callback": {
						"address": "%s",
						"gas_limit": "0",
						"calldata": ""
					}
				}`, contractAddr)
			},
			expError: "ABCI code: 8",
		},

		// FAILURE CASES - Invalid Memo Format
		{
			name:     "failure - missing required callback fields",
			malleate: nil,
			memo: func() string {
				return `{"dest_callback": {"address": ""}}`
			},
			expError: "a",
		},
		{
			name:     "failure - invalid callback address format",
			malleate: nil,
			memo: func() string {
				return `{"dest_callback": {"address": "not_hex_address", "gas_limit": "1000000", "calldata": ""}}`
			},
			expError: "a",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Reset state for each test
			suite.SetupTest()
			path = suite.path

			ctxB := suite.chainB.GetContext()
			evmCtx := suite.evmChainA.GetContext()
			bondDenom, err := suite.chainB.GetSimApp().StakingKeeper.BondDenom(ctxB)
			suite.Require().NoError(err)

			// Generate the isolated address for the sender
			sendAmt := ibctesting.DefaultCoinAmount
			isolatedAddr := types2.GenerateIsolatedAddress(path.EndpointA.ChannelID, suite.chainB.SenderAccount.GetAddress().String())

			// Get callback tester contract and deploy it
			contractData, err = testutil2.LoadCounterWithCallbacksContract()
			suite.Require().NoError(err)

			deploymentData := testutiltypes.ContractDeploymentData{
				Contract:        contractData,
				ConstructorArgs: nil,
			}

			contractAddr, err = DeployContract(suite.T(), suite.evmChainA, deploymentData)
			suite.Require().NoError(err)

			// Generate packet to execute the tester contract using callbacks
			packetData := transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				sendAmt.String(),
				suite.chainB.SenderAccount.GetAddress().String(),
				isolatedAddr.String(),
				"", // Will be set by memo function
			)

			_ = path.EndpointA.GetChannel()
			sourceChan := path.EndpointB.GetChannel()

			unmarshalledData, ackErr := transfertypes.UnmarshalPacketData(packetData.GetBytes(), sourceChan.Version, "")
			data = unmarshalledData
			suite.Require().Nil(ackErr)

			voucherDenom = testutil.GetVoucherDenomFromPacketData(data, path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)

			// Set the memo from the test case
			packetData.Memo = tc.memo()

			// Apply test-specific setup
			if tc.malleate != nil {
				tc.malleate()
			}

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

			// Get transfer stack
			transferStack, ok := suite.evmChainA.App.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
			suite.Require().True(ok)

			_, found := suite.evmChainA.App.GetIBCKeeper().ChannelKeeper.GetChannel(evmCtx, path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
			suite.Require().True(found)

			// Execute the packet
			ack := transferStack.OnRecvPacket(
				evmCtx,
				sourceChan.Version,
				packet,
				suite.evmChainA.SenderAccount.GetAddress(),
			)

			// Validate successful callback
			evmApp := suite.evmChainA.App.(*evmd.EVMD)
			singleTokenRepresentation, err := types.NewTokenPairSTRv2(voucherDenom)
			suite.Require().NoError(err)
			erc20Contract := singleTokenRepresentation.GetERC20Contract()

			// Validate results
			if tc.expError == "" {
				suite.Require().True(ack.Success(), "Expected success but got failure")

				balAfterCallback := evmApp.Erc20Keeper.BalanceOf(evmCtx, contracts.ERC20MinterBurnerDecimalsContract.ABI, erc20Contract, contractAddr)
				suite.Require().Equal(sendAmt.String(), balAfterCallback.String())

				tokenPair, found := evmApp.Erc20Keeper.GetTokenPair(evmCtx, singleTokenRepresentation.GetID())
				suite.Require().True(found)
				suite.Require().Equal(voucherDenom, tokenPair.Denom)

				available := evmApp.Erc20Keeper.IsDynamicPrecompileAvailable(evmCtx, common.HexToAddress(tokenPair.Erc20Address))
				suite.Require().True(available)
			} else {
				suite.Require().False(ack.Success(), "Expected failure but got success")

				balAfterCallback := evmApp.Erc20Keeper.BalanceOf(evmCtx, contracts.ERC20MinterBurnerDecimalsContract.ABI, erc20Contract, contractAddr)
				suite.Require().Equal("0", balAfterCallback.String())

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
				available := evmApp.Erc20Keeper.IsDynamicPrecompileAvailable(ctxA, common.HexToAddress(tokenPair.Erc20Address))
				suite.Require().True(available)
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
	nativeErc20 := SetupNativeErc20(suite.T(), suite.evmChainA, suite.evmChainA.SenderAccounts[0])

	evmCtx := suite.evmChainA.GetContext()
	evmApp := suite.evmChainA.App.(*evmd.EVMD)

	// Scenario: Native ERC20 token transfer from evmChainA to chainB
	timeoutHeight := clienttypes.NewHeight(1, 110)
	path := suite.path
	chainBAccount := suite.chainB.SenderAccount.GetAddress()

	sendAmt := math.NewIntFromBigInt(nativeErc20.InitialBal).Quo(math.NewInt(2))
	senderEthAddr := nativeErc20.Account
	sender := sdk.AccAddress(senderEthAddr.Bytes())

	// Transfer half the initial balance out
	// Sender transfers 50 out (escrowed)
	msg := transfertypes.NewMsgTransfer(
		path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID,
		sdk.NewCoin(nativeErc20.Denom, sendAmt),
		sender.String(), chainBAccount.String(),
		timeoutHeight, 0, "",
	)

	_, err := suite.evmChainA.SendMsgs(msg)
	suite.Require().NoError(err) // message committed

	// Balance after transfer should be initial balance - sendAmt
	balAfterTransfer := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, senderEthAddr)
	suite.Require().Equal(
		new(big.Int).Sub(nativeErc20.InitialBal, sendAmt.BigInt()).String(),
		balAfterTransfer.String(),
	)

	// Now try to convert sendAmt to ERC20
	convertMsg := types.MsgConvertERC20{
		ContractAddress: nativeErc20.ContractAddr.String(),
		Amount:          sendAmt,
		Receiver:        sender.String(),
		Sender:          senderEthAddr.String(),
	}

	_, err = suite.evmChainA.SendMsgs(&convertMsg)
	suite.Require().NoError(err) // message committed

	// Check native erc20 token is escrowed on evmChainA for sending to chainB.
	// Conversion of remaining 50 tokens to Bank token
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

	// half the send amount should be received since our first call will fail due to send disabled,
	// and the second will succeed
	recvAmt := sendAmt.Quo(math.NewInt(2))
	// Mock the transfer of received native erc20 token by evmChainA to evmChainA.
	// Note that ChainB didn't receive the native erc20 token. We just assume that.
	packetData := transfertypes.NewFungibleTokenPacketData(
		chainBNativeErc20Denom.Path(),
		recvAmt.String(),
		chainBAccount.String(),
		types2.GenerateIsolatedAddress(path.EndpointA.ChannelID, suite.chainB.SenderAccount.GetAddress().String()).String(),
		"",
	)

	// get callback tester contract and deploy it
	contractData, err := testutil2.LoadCounterWithCallbacksContract()
	suite.Require().NoError(err)

	deploymentData := testutiltypes.ContractDeploymentData{
		Contract:        contractData,
		ConstructorArgs: nil,
	}

	contractAddr, err := DeployContract(suite.T(), suite.evmChainA, deploymentData)
	if err != nil {
		return
	}

	// Each callback gets recvAmt
	packedBytes, err := contractData.ABI.Pack("add", nativeErc20.ContractAddr, recvAmt.BigInt())
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

	// SendEnabled=false will cause the conversion of bank tokens to erc20 tokens to fail,
	// but not send them back to escrow
	evmApp.BankKeeper.SetSendEnabled(evmCtx, nativeErc20.Denom, false)
	isSendEnabled := evmApp.BankKeeper.IsSendEnabledDenom(evmCtx, nativeErc20.Denom)
	suite.Require().False(isSendEnabled)

	errAck := transferStack.OnRecvPacket(
		evmCtx,
		sourceChan.Version,
		packet,
		suite.evmChainA.SenderAccount.GetAddress(),
	)
	suite.Require().False(errAck.Success())

	evmCtx = suite.evmChainA.GetContext()

	// SendEnabled=true causes our callback to succeed
	evmApp.BankKeeper.SetSendEnabled(evmCtx, nativeErc20.Denom, true)
	isSendEnabled = evmApp.BankKeeper.IsSendEnabledDenom(evmCtx, nativeErc20.Denom)
	suite.Require().True(isSendEnabled)

	packet2 := channeltypes.Packet{
		Sequence:           2,
		SourcePort:         path.EndpointB.ChannelConfig.PortID,
		SourceChannel:      path.EndpointB.ChannelID,
		DestinationPort:    path.EndpointA.ChannelConfig.PortID,
		DestinationChannel: path.EndpointA.ChannelID,
		Data:               packetData.GetBytes(),
		TimeoutHeight:      suite.evmChainA.GetTimeoutHeight(),
		TimeoutTimestamp:   0,
	}

	ack := transferStack.OnRecvPacket(
		evmCtx,
		sourceChan.Version,
		packet2,
		suite.evmChainA.SenderAccount.GetAddress(),
	)
	suite.Require().True(ack.Success())

	// Check un-escrowed balance on evmChainA after receiving the packet.
	escrowedBal = evmApp.BankKeeper.GetBalance(evmCtx, escrowAddr, nativeErc20.Denom)
	suite.Require().True(escrowedBal.IsZero(), "escrowed balance should be un-escrowed after receiving the packet")
	// recvAmt should be in the contractAddr upon successful recv callback
	balAfterUnescrow := evmApp.Erc20Keeper.BalanceOf(evmCtx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, contractAddr)
	suite.Require().Equal(recvAmt.String(), balAfterUnescrow.String())
	bankBalAfterUnescrow := evmApp.BankKeeper.GetBalance(evmCtx, sender, nativeErc20.Denom)
	// InitialBalance half which was converted but not sent will be in the sending account's balance
	suite.Require().Equal(sendAmt.String(), bankBalAfterUnescrow.Amount.String())

	// the packet that failed conversion due to the minting restriction should instead remain as the bank token
	// and will be in the isolated address used to invoke the callback
	trappedBal := evmApp.BankKeeper.GetBalance(evmCtx, types2.GenerateIsolatedAddress(path.EndpointA.ChannelID,
		suite.chainB.SenderAccount.GetAddress().String()), nativeErc20.Denom)
	suite.Require().Equal(recvAmt.String(), trappedBal.Amount.String())
}

// TestOnAcknowledgementPacketWithCallback tests acknowledgement logic with comprehensive callback scenarios.
func (suite *MiddlewareTestSuite) TestOnAcknowledgementPacketWithCallback() {
	var (
		packet       channeltypes.Packet
		ack          []byte
		contractData types3.CompiledContract
		contractAddr common.Address
	)

	testCases := []struct {
		name           string
		malleate       func()
		memo           func() string
		ackType        string // "success" or "error"
		onSendRequired bool
		expError       string
	}{
		// SUCCESS CASES
		{
			name:     "success - callback with successful acknowledgement",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d"
					}
				}`, contractAddr, 1_000_000)
			},
			ackType:        "success",
			onSendRequired: true,
			expError:       "",
		},
		{
			name:     "success - callback with error acknowledgement (refund scenario)",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d"
					}
				}`, contractAddr, 1_500_000)
			},
			ackType:        "error",
			onSendRequired: true,
			expError:       "",
		},
		{
			name:     "success - callback with maximum gas limit",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d"
					}
				}`, contractAddr, 10_000_000)
			},
			ackType:        "success",
			onSendRequired: true,
			expError:       "",
		},
		{
			name:     "success - no callback in memo (regular transfer)",
			malleate: nil,
			memo: func() string {
				return ""
			},
			ackType:        "success",
			onSendRequired: true,
			expError:       "",
		},

		// FAILURE CASES - Invalid Contract
		{
			name:     "failure - callback to non-existent contract",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "0x1234567890123456789012345678901234567890",
						"gas_limit": "%d"
					}
				}`, 1_000_000)
			},
			ackType:        "success",
			onSendRequired: true,
			expError:       "ABCI code: 4",
		},
		{
			name:     "failure - callback to empty address",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "0x0000000000000000000000000000000000000000",
						"gas_limit": "%d"
					}
				}`, 1_000_000)
			},
			ackType:        "success",
			onSendRequired: true,
			expError:       "ABCI code: 4",
		},

		// FAILURE CASES - Invalid Calldata
		{
			name:     "failure - acknowledgement callback with calldata (should be empty)",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d",
						"calldata": "%x"
					}
				}`, contractAddr, 1_000_000, []byte{0x12, 0x34, 0x56, 0x78})
			},
			ackType:        "success",
			onSendRequired: true,
			expError:       "ABCI code: 3",
		},

		// FAILURE CASES - Gas Issues
		{
			name:     "failure - insufficient gas limit",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d"
					}
				}`, contractAddr, 1000) // Very low gas
			},
			ackType:        "success",
			onSendRequired: true,
			expError:       "ABCI code: 9",
		},
		{
			name:     "success - zero gas limit (defaults to max)",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "0"
					}
				}`, contractAddr)
			},
			ackType:        "success",
			onSendRequired: true,
			expError:       "",
		},

		// FAILURE CASES - Invalid Memo Format
		{
			name:     "failure - malformed JSON memo",
			malleate: nil,
			memo: func() string {
				return `{"src_callback": {"address": "invalid_json"`
			},
			ackType:        "success",
			onSendRequired: true,
			expError:       "invalid callback data",
		},
		{
			name:     "failure - invalid callback address format",
			malleate: nil,
			memo: func() string {
				return `{"src_callback": {"address": "not_hex_address", "gas_limit": "1000000"}}`
			},
			ackType:        "success",
			onSendRequired: true,
			expError:       "invalid callback data",
		},

		// FAILURE CASES - Base IBC Failures (should not execute callback)
		{
			name: "failure - malformed packet data (no callback execution)",
			malleate: func() {
				packet.Data = []byte("malformed data")
			},
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d"
					}
				}`, contractAddr, 1_000_000)
			},
			ackType:        "success",
			onSendRequired: false,
			expError:       "cannot unmarshal ICS-20 transfer packet data",
		},
		{
			name: "failure - empty acknowledgement (no callback execution)",
			malleate: func() {
				ack = []byte{}
			},
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d"
					}
				}`, contractAddr, 1_000_000)
			},
			ackType:        "success",
			onSendRequired: true,
			expError:       "cannot unmarshal ICS-20 transfer packet acknowledgement",
		},

		// EDGE CASES
		{
			name:     "success - callback with error ack and refund verification",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d"
					}
				}`, contractAddr, 2_000_000)
			},
			ackType:        "error",
			onSendRequired: true,
			expError:       "",
		},
		{
			name:     "success - callback with minimal gas",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d"
					}
				}`, contractAddr, 100_000) // Minimal but sufficient
			},
			ackType:        "success",
			onSendRequired: true,
			expError:       "",
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

			// Deploy callback contract on source chain (evmChainA)
			contractData, err = testutil2.LoadCounterWithCallbacksContract()
			suite.Require().NoError(err)

			deploymentData := testutiltypes.ContractDeploymentData{
				Contract:        contractData,
				ConstructorArgs: nil,
			}

			contractAddr, err = DeployContract(suite.T(), suite.evmChainA, deploymentData)
			suite.Require().NoError(err)

			// Create packet data with memo
			packetData := transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				sendAmt.String(),
				sender.String(),
				receiver.String(),
				tc.memo(),
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

			// Set acknowledgement based on test case
			if tc.ackType == "error" {
				ackErr := channeltypes.NewErrorAcknowledgement(errors.New("transfer failed"))
				ack = ackErr.Acknowledgement()
			} else {
				ack = channeltypes.NewResultAcknowledgement([]byte{1}).Acknowledgement()
			}

			// Apply test-specific malleate function
			if tc.malleate != nil {
				tc.malleate()
			}

			transferStack, ok := evmApp.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
			suite.Require().True(ok)

			sourceChan := suite.path.EndpointA.GetChannel()

			balBeforeTransfer := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)
			// Execute send if required (for proper escrow setup)
			if tc.onSendRequired {
				timeoutHeight := clienttypes.NewHeight(1, 110)
				msg := transfertypes.NewMsgTransfer(
					path.EndpointA.ChannelConfig.PortID,
					path.EndpointA.ChannelID,
					sdk.NewCoin(bondDenom, sendAmt),
					sender.String(),
					receiver.String(),
					timeoutHeight, 0, tc.memo(),
				)
				err = suite.evmChainA.SenderAccount.SetSequence(suite.evmChainA.SenderAccount.GetSequence() + 1)
				suite.Require().NoError(err)
				res, err := suite.evmChainA.SendMsgs(msg)
				suite.Require().NoError(err) // message committed

				feeAmt := evmibctesting.FeeCoins().AmountOf(bondDenom)
				balAfterTransfer := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)
				suite.Require().Equal(
					balBeforeTransfer.Amount.Sub(sendAmt).Sub(feeAmt).String(),
					balAfterTransfer.Amount.String(),
				)

				sentPacket, err := ibctesting.ParseV1PacketFromEvents(res.Events)
				suite.Require().NoError(err)

				// relay the sent packet
				err = path.RelayPacket(sentPacket)
				suite.Require().NoError(err) // relay committed

				// Verify escrow for successful sends
				if tc.expError == "" || !strings.Contains(tc.expError, "ABCI code") {
					// One for UpdateClient() and one for AcknowledgePacket()
					relayPacketFeeAmt := feeAmt.Mul(math.NewInt(2))

					balAfterRelayPacket := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)
					suite.Require().Equal(
						balAfterTransfer.Amount.Sub(relayPacketFeeAmt).String(),
						balAfterRelayPacket.Amount.String(),
					)
					escrowAddr := transfertypes.GetEscrowAddress(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
					escrowedBal := evmApp.BankKeeper.GetBalance(ctxA, escrowAddr, bondDenom)
					suite.Require().Equal(sendAmt.String(), escrowedBal.Amount.String())
				}

				// Use the actually sent packet for acknowledgement
				packet = sentPacket
			}

			beforeAckBal := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)
			// Execute acknowledgement
			err = transferStack.OnAcknowledgementPacket(
				ctxA,
				sourceChan.Version,
				packet,
				ack,
				receiver,
			)

			// Validate results
			if tc.expError == "" {
				suite.Require().NoError(err, "Expected success but got error")

				// Verify callback execution by checking counter increment
				if strings.Contains(tc.memo(), "src_callback") {
					counterRes, err := evmApp.EVMKeeper.CallEVM(
						ctxA,
						contractData.ABI,
						common.BytesToAddress(suite.evmChainA.SenderAccount.GetAddress()),
						contractAddr,
						false,
						big.NewInt(100000),
						"getCounter",
					)
					suite.Require().NoError(err)

					var counter *big.Int
					err = contractData.ABI.UnpackIntoInterface(&counter, "getCounter", counterRes.Ret)
					suite.Require().NoError(err)
					suite.Require().True(counter.Cmp(big.NewInt(1)) >= 0, "Counter should be incremented by callback")
				}

				// Verify refund for error acknowledgements
				if tc.ackType == "error" && tc.onSendRequired {
					escrowAddr := transfertypes.GetEscrowAddress(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
					escrowedBal := evmApp.BankKeeper.GetBalance(ctxA, escrowAddr, bondDenom)
					finalSenderBal := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)

					// For error acks, tokens should be refunded
					suite.Require().True(escrowedBal.IsZero(), "Escrowed balance should be zero after refund")
					suite.Require().Equal(beforeAckBal.Amount.Add(sendAmt).String(), finalSenderBal.Amount.String(), "Sender balance should be refunded")
				}
			} else if strings.Contains(tc.memo(), "src_callback") && strings.Contains(tc.expError, "ABCI code") {
				// For ack failures, verify that counter was NOT incremented

				counterRes, err := evmApp.EVMKeeper.CallEVM(
					ctxA,
					contractData.ABI,
					common.BytesToAddress(suite.evmChainA.SenderAccount.GetAddress()),
					contractAddr,
					false,
					big.NewInt(100000),
					"getCounter",
				)
				// Counter should remain 0 if callback failed
				if err == nil {
					var counter *big.Int
					err = contractData.ABI.UnpackIntoInterface(&counter, "getCounter", counterRes.Ret)
					if err == nil {
						suite.Require().Equal(big.NewInt(0).String(), counter.String(), "Counter should not be incremented on callback failure")
					}
				}
			}
		})
	}
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
				balBeforeTransfer := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)
				res, err := suite.evmChainA.SendMsgs(msg)
				suite.Require().NoError(err) // message committed

				feeAmt := evmibctesting.FeeCoins().AmountOf(bondDenom)
				balAfterTransfer := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)
				suite.Require().Equal(
					balBeforeTransfer.Amount.Sub(sendAmt).Sub(feeAmt).String(),
					balAfterTransfer.Amount.String(),
				)

				packet, err := ibctesting.ParseV1PacketFromEvents(res.Events)
				suite.Require().NoError(err)

				// relay the sent packet
				err = path.RelayPacket(packet)
				suite.Require().NoError(err) // relay committed

				// One for UpdateClient() and one for AcknowledgePacket()
				relayPacketFeeAmt := feeAmt.Mul(math.NewInt(2))

				// ensure the ibc token is escrowed.
				balAfterRelayPacket := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)
				suite.Require().Equal(
					balAfterTransfer.Amount.Sub(relayPacketFeeAmt).String(),
					balAfterRelayPacket.Amount.String(),
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
			nativeErc20 := SetupNativeErc20(suite.T(), suite.evmChainA, suite.evmChainA.SenderAccounts[0])

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

			balBeforeTransfer := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)
			var balAfterRelayPacket sdk.Coin
			feeAmt := evmibctesting.FeeCoins().AmountOf(bondDenom)
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

				balAfterTransfer := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)
				suite.Require().Equal(
					balBeforeTransfer.Amount.Sub(sendAmt).Sub(feeAmt).String(),
					balAfterTransfer.Amount.String(),
				)

				packet, err := ibctesting.ParseV1PacketFromEvents(res.Events)
				suite.Require().NoError(err)

				err = path.RelayPacket(packet)
				suite.Require().NoError(err) // relay committed

				// One for UpdateClient() and one for AcknowledgePacket()
				relayPacketFeeAmt := feeAmt.Mul(math.NewInt(2))

				balAfterRelayPacket = evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)
				suite.Require().Equal(
					balAfterTransfer.Amount.Sub(relayPacketFeeAmt).String(),
					balAfterRelayPacket.Amount.String(),
				)
			}
			err = onTimeout()

			balAfterTimeout := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)
			if tc.onSendRequired {
				suite.Require().Equal(
					balAfterRelayPacket.Amount.Add(sendAmt).String(),
					balAfterTimeout.Amount.String(),
				)
			} else {
				suite.Require().Equal(
					balBeforeTransfer.Amount.String(),
					balAfterTimeout.Amount.String(),
				)
			}

			// ensure that the escrowed coins were refunded on timeout.
			escrowAddr := transfertypes.GetEscrowAddress(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
			escrowedBal := evmApp.BankKeeper.GetBalance(ctxA, escrowAddr, bondDenom)
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

// TestOnTimeoutPacketWithCallback tests timeout logic with comprehensive callback scenarios.
func (suite *MiddlewareTestSuite) TestOnTimeoutPacketWithCallback() {
	var (
		packet       channeltypes.Packet
		contractData types3.CompiledContract
		contractAddr common.Address
	)

	testCases := []struct {
		name           string
		malleate       func()
		memo           func() string
		onSendRequired bool
		expError       string
	}{
		// SUCCESS CASES
		{
			name:     "success - callback with timeout",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d"
					}
				}`, contractAddr, 1_000_000)
			},
			onSendRequired: true,
			expError:       "",
		},
		{
			name:     "success - callback with maximum gas limit",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d"
					}
				}`, contractAddr, 10_000_000)
			},
			onSendRequired: true,
			expError:       "",
		},
		{
			name:     "success - no callback in memo (regular timeout)",
			malleate: nil,
			memo: func() string {
				return ""
			},
			onSendRequired: true,
			expError:       "",
		},

		// FAILURE CASES - Invalid Contract
		{
			name:     "failure - callback to non-existent contract",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "0x1234567890123456789012345678901234567890",
						"gas_limit": "%d"
					}
				}`, 1_000_000)
			},
			onSendRequired: true,
			expError:       "ABCI code: 4",
		},
		{
			name:     "failure - callback to empty address",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "0x0000000000000000000000000000000000000000",
						"gas_limit": "%d"
					}
				}`, 1_000_000)
			},
			onSendRequired: true,
			expError:       "ABCI code: 4",
		},

		// FAILURE CASES - Invalid Calldata
		{
			name:     "failure - timeout callback with calldata (should be empty)",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d",
						"calldata": "%x"
					}
				}`, contractAddr, 1_000_000, []byte{0xab, 0xcd, 0xef, 0x12})
			},
			onSendRequired: true,
			expError:       "ABCI code: 3",
		},

		// FAILURE CASES - Gas Issues
		{
			name:     "failure - insufficient gas limit",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d"
					}
				}`, contractAddr, 1000) // Very low gas
			},
			onSendRequired: true,
			expError:       "ABCI code: 9",
		},
		{
			name:     "success - zero gas limit (defaults to max)",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "0"
					}
				}`, contractAddr)
			},
			onSendRequired: true,
			expError:       "",
		},

		// FAILURE CASES - Invalid Memo Format
		{
			name:     "failure - malformed JSON memo",
			malleate: nil,
			memo: func() string {
				return `{"src_callback": {"address": "invalid_json"`
			},
			onSendRequired: true,
			expError:       "invalid callback data",
		},
		{
			name:     "failure - invalid callback address format",
			malleate: nil,
			memo: func() string {
				return `{"src_callback": {"address": "not_hex_address", "gas_limit": "1000000"}}`
			},
			onSendRequired: true,
			expError:       "invalid callback data",
		},

		// FAILURE CASES - Base IBC Failures (should not execute callback)
		{
			name: "failure - malformed packet data (no callback execution)",
			malleate: func() {
				packet.Data = []byte("malformed data")
			},
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d"
					}
				}`, contractAddr, 1_000_000)
			},
			onSendRequired: false,
			expError:       "cannot unmarshal ICS-20 transfer packet data",
		},

		// EDGE CASES
		{
			name:     "failure - callback with minimal gas",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d"
					}
				}`, contractAddr, 1000) // Minimal and insufficient
			},
			onSendRequired: true,
			expError:       "out of gas",
		},
		{
			name:     "success - timeout with refund verification",
			malleate: nil,
			memo: func() string {
				return fmt.Sprintf(`{
					"src_callback": {
						"address": "%s",
						"gas_limit": "%d"
					}
				}`, contractAddr, 2_000_000)
			},
			onSendRequired: true,
			expError:       "",
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

			// Deploy callback contract on source chain (evmChainA)
			contractData, err = testutil2.LoadCounterWithCallbacksContract()
			suite.Require().NoError(err)

			deploymentData := testutiltypes.ContractDeploymentData{
				Contract:        contractData,
				ConstructorArgs: nil,
			}

			contractAddr, err = DeployContract(suite.T(), suite.evmChainA, deploymentData)
			suite.Require().NoError(err)

			// Create packet data with memo
			packetData := transfertypes.NewFungibleTokenPacketData(
				bondDenom,
				sendAmt.String(),
				sender.String(),
				receiver.String(),
				tc.memo(),
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

			// Apply test-specific malleate function
			if tc.malleate != nil {
				tc.malleate()
			}

			transferStack, ok := evmApp.GetIBCKeeper().PortKeeper.Route(transfertypes.ModuleName)
			suite.Require().True(ok)

			balBeforeTransfer := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)
			balAfterTransfer := balBeforeTransfer
			feeAmt := evmibctesting.FeeCoins().AmountOf(bondDenom)
			// Execute send if required (for proper escrow setup)
			if tc.onSendRequired {
				timeoutHeight := clienttypes.NewHeight(1, 110)
				msg := transfertypes.NewMsgTransfer(
					path.EndpointA.ChannelConfig.PortID,
					path.EndpointA.ChannelID,
					sdk.NewCoin(bondDenom, sendAmt),
					sender.String(),
					receiver.String(),
					timeoutHeight, 0, tc.memo(),
				)
				err = suite.evmChainA.SenderAccount.SetSequence(suite.evmChainA.SenderAccount.GetSequence() + 1)
				suite.Require().NoError(err)
				res, err := suite.evmChainA.SendMsgs(msg)
				suite.Require().NoError(err) // message committed

				sentPacket, err := ibctesting.ParseV1PacketFromEvents(res.Events)
				suite.Require().NoError(err)

				balAfterTransfer = evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)
				suite.Require().Equal(
					balBeforeTransfer.Amount.Sub(sendAmt).Sub(feeAmt).String(),
					balAfterTransfer.Amount.String(),
				)
				escrowAddr := transfertypes.GetEscrowAddress(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
				escrowedBal := evmApp.BankKeeper.GetBalance(ctxA, escrowAddr, bondDenom)
				suite.Require().Equal(sendAmt.String(), escrowedBal.Amount.String())

				// Use the actually sent packet for timeout
				packet = sentPacket
			}

			sourceChan := path.EndpointA.GetChannel()
			err = transferStack.OnTimeoutPacket(
				ctxA,
				sourceChan.Version,
				packet,
				receiver,
			)
			balAfterTimeout := evmApp.BankKeeper.GetBalance(ctxA, sender, bondDenom)

			// Validate results
			if tc.expError == "" {
				suite.Require().NoError(err, "Expected success but got error")

				// Verify callback execution by checking that counter was NOT decremented
				// The onPacketTimeout function in the contract doesn't modify the counter,
				// so we verify the callback was executed by checking the counter remains unchanged
				if strings.Contains(tc.memo(), "src_callback") {
					counterRes, err := evmApp.EVMKeeper.CallEVM(
						ctxA,
						contractData.ABI,
						common.BytesToAddress(suite.evmChainA.SenderAccount.GetAddress()),
						contractAddr,
						false,
						big.NewInt(100000),
						"getCounter",
					)
					suite.Require().NoError(err)

					var counter *big.Int
					err = contractData.ABI.UnpackIntoInterface(&counter, "getCounter", counterRes.Ret)
					suite.Require().NoError(err)

					// For timeout callbacks, counter should be -1
					// This verifies the callback was executed without error, but didn't change the counter
					suite.Require().Equal(big.NewInt(-1).String(), counter.String(), "Counter should be -1 for timeout callbacks")
				}

				// Verify refund for timeouts (tokens should always be refunded on timeout)
				if tc.onSendRequired {
					escrowAddr := transfertypes.GetEscrowAddress(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
					escrowedBal := evmApp.BankKeeper.GetBalance(ctxA, escrowAddr, bondDenom)

					// For timeouts, tokens should always be refunded
					suite.Require().True(escrowedBal.IsZero(), "Escrowed balance should be zero after timeout refund")
					suite.Require().Equal(balAfterTransfer.Amount.Add(sendAmt).String(), balAfterTimeout.Amount.String(), "Sender balance should be refunded on timeout")
				}
			} else {
				// For timeout callback failures, verify that counter was NOT changed
				if strings.Contains(tc.memo(), "src_callback") && strings.Contains(tc.expError, "ABCI code") {
					counterRes, err := evmApp.EVMKeeper.CallEVM(
						ctxA,
						contractData.ABI,
						common.BytesToAddress(suite.evmChainA.SenderAccount.GetAddress()),
						contractAddr,
						false,
						big.NewInt(100000),
						"getCounter",
					)
					// Counter should remain 0 if callback failed
					if err == nil {
						var counter *big.Int
						err = contractData.ABI.UnpackIntoInterface(&counter, "getCounter", counterRes.Ret)
						if err == nil {
							suite.Require().Equal(big.NewInt(0).String(), counter.String(), "Counter should remain 0 on timeout callback failure")
						}
					}
				}

				// For timeout callback failures, the base timeout logic should still work
				// unless it's a fundamental packet data issue
				if tc.onSendRequired && !strings.Contains(tc.expError, "cannot unmarshal") {
					// Even if callback fails, the timeout refund should still happen
					escrowAddr := transfertypes.GetEscrowAddress(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
					escrowedBal := evmApp.BankKeeper.GetBalance(ctxA, escrowAddr, bondDenom)

					suite.Require().True(escrowedBal.IsZero(), "Escrowed balance should be zero after timeout refund even with callback failure")
					suite.Require().Equal(balAfterTransfer.Amount.Add(sendAmt).String(), balAfterTimeout.Amount.String(), "Sender balance should be refunded on timeout")
				}
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
			nativeErc20 := SetupNativeErc20(suite.T(), suite.evmChainA, suite.evmChainA.SenderAccounts[0])

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
