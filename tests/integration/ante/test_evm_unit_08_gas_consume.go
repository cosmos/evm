package ante

import (
	"fmt"

	evmante "github.com/cosmos/evm/ante/evm"
	testconstants "github.com/cosmos/evm/testutil/constants"
	commonfactory "github.com/cosmos/evm/testutil/integration/base/factory"
	testfactory "github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func (s *EvmUnitAnteTestSuite) TestUpdateCumulativeGasWanted() {
	keyring := testkeyring.New(1)
	unitNetwork := network.NewUnitTestNetwork(
		s.create,
		network.WithChainID(testconstants.ChainID{
			ChainID:    s.ChainID,
			EVMChainID: s.EvmChainID,
		}),
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	)

	testCases := []struct {
		name                string
		msgGasWanted        uint64
		maxTxGasWanted      uint64
		cumulativeGasWanted uint64
		getCtx              func() sdktypes.Context
		expectedResponse    uint64
	}{
		{
			name:                "when is NOT checkTx and cumulativeGasWanted is 0, returns msgGasWanted",
			msgGasWanted:        100,
			maxTxGasWanted:      150,
			cumulativeGasWanted: 0,
			getCtx: func() sdktypes.Context {
				return unitNetwork.GetContext().WithIsCheckTx(false)
			},
			expectedResponse: 100,
		},
		{
			name:                "when is NOT checkTx and cumulativeGasWanted has value, returns cumulativeGasWanted + msgGasWanted",
			msgGasWanted:        100,
			maxTxGasWanted:      150,
			cumulativeGasWanted: 50,
			getCtx: func() sdktypes.Context {
				return unitNetwork.GetContext().WithIsCheckTx(false)
			},
			expectedResponse: 150,
		},
		{
			name:                "when is checkTx, maxTxGasWanted is not 0 and msgGasWanted > maxTxGasWanted, returns cumulativeGasWanted + maxTxGasWanted",
			msgGasWanted:        200,
			maxTxGasWanted:      100,
			cumulativeGasWanted: 50,
			getCtx: func() sdktypes.Context {
				return unitNetwork.GetContext().WithIsCheckTx(true)
			},
			expectedResponse: 150,
		},
		{
			name:                "when is checkTx, maxTxGasWanted is not 0 and msgGasWanted < maxTxGasWanted, returns cumulativeGasWanted + msgGasWanted",
			msgGasWanted:        50,
			maxTxGasWanted:      100,
			cumulativeGasWanted: 50,
			getCtx: func() sdktypes.Context {
				return unitNetwork.GetContext().WithIsCheckTx(true)
			},
			expectedResponse: 100,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Function under test
			gasWanted := evmante.UpdateCumulativeGasWanted(
				tc.getCtx(),
				tc.msgGasWanted,
				tc.maxTxGasWanted,
				tc.cumulativeGasWanted,
			)

			s.Require().Equal(tc.expectedResponse, gasWanted)
		})
	}
}

// NOTE: claim rewards are not tested since there is an independent suite to test just that
func (s *EvmUnitAnteTestSuite) TestConsumeGasAndEmitEvent() {
	keyring := testkeyring.New(1)
	unitNetwork := network.NewUnitTestNetwork(
		s.create,
		network.WithChainID(testconstants.ChainID{
			ChainID:    s.ChainID,
			EVMChainID: s.EvmChainID,
		}),
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	)
	grpcHandler := grpc.NewIntegrationHandler(unitNetwork)
	factory := testfactory.New(unitNetwork, grpcHandler)

	testCases := []struct {
		name          string
		expectedError string
		feesAmt       sdkmath.Int
		getSender     func() sdktypes.AccAddress
	}{
		{
			name:    "success: fees are zero and event emitted",
			feesAmt: sdkmath.NewInt(0),
			getSender: func() sdktypes.AccAddress {
				// Return prefunded sender
				return keyring.GetKey(0).AccAddr
			},
		},
		{
			name:    "success: there are non zero fees, user has sufficient bank balances and event emitted",
			feesAmt: sdkmath.NewInt(1000),
			getSender: func() sdktypes.AccAddress {
				// Return prefunded sender
				return keyring.GetKey(0).AccAddr
			},
		},
		{
			name:          "fail: insufficient user balance, event is NOT emitted",
			expectedError: "failed to deduct transaction costs from user balance",
			feesAmt:       sdkmath.NewInt(1000),
			getSender: func() sdktypes.AccAddress {
				// Set up account with too little balance (but not zero)
				index := keyring.AddKey()
				acc := keyring.GetKey(index)

				sender := keyring.GetKey(0)
				_, err := factory.ExecuteCosmosTx(sender.Priv, commonfactory.CosmosTxArgs{
					Msgs: []sdktypes.Msg{&banktypes.MsgSend{
						FromAddress: sender.AccAddr.String(),
						ToAddress:   acc.AccAddr.String(),
						Amount:      sdktypes.Coins{sdktypes.NewCoin(unitNetwork.GetBaseDenom(), sdkmath.NewInt(500))},
					}},
				})
				s.Require().NoError(err, "failed to send funds to new key")

				return acc.AccAddr
			},
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("%v_%v_%v", evmtypes.GetTxTypeName(s.EthTxType), s.ChainID, tc.name), func() {
			sender := tc.getSender()

			resp, err := grpcHandler.GetBalanceFromEVM(sender)
			s.Require().NoError(err)
			prevBalance, ok := sdkmath.NewIntFromString(resp.Balance)
			s.Require().True(ok)

			evmDecimals := evmtypes.GetEVMCoinDecimals()
			feesAmt := tc.feesAmt.Mul(evmDecimals.ConversionFactor())
			fees := sdktypes.NewCoins(sdktypes.NewCoin(unitNetwork.GetBaseDenom(), feesAmt))

			// Function under test
			err = evmante.ConsumeFeesAndEmitEvent(
				unitNetwork.GetContext(),
				unitNetwork.App.GetEVMKeeper(),
				fees,
				sender,
			)

			if tc.expectedError != "" {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.expectedError)

				// Check events are not present
				events := unitNetwork.GetContext().EventManager().Events()
				s.Require().Zero(len(events), "required no events to be emitted")
			} else {
				s.Require().NoError(err)

				// Check fees are deducted
				resp, err := grpcHandler.GetBalanceFromEVM(sender)
				s.Require().NoError(err)
				afterBalance, ok := sdkmath.NewIntFromString(resp.Balance)
				s.Require().True(ok)

				s.Require().NoError(err)
				expectedBalance := prevBalance.Sub(feesAmt)
				s.Require().True(expectedBalance.Equal(afterBalance), "expected different balance after fees deduction")

				// Event to be emitted
				expectedEvent := sdktypes.NewEvent(
					sdktypes.EventTypeTx,
					sdktypes.NewAttribute(sdktypes.AttributeKeyFee, fees.String()),
				)
				// Check events are present
				events := unitNetwork.GetContext().EventManager().Events()
				s.Require().NotZero(len(events))
				s.Require().Contains(
					events,
					expectedEvent,
					"expected different events after fees deduction",
				)
			}

			// Reset the context
			err = unitNetwork.NextBlock()
			s.Require().NoError(err)
		})
	}
}
