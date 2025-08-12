package ante

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/evm/ante/evm"
	testconstants "github.com/cosmos/evm/testutil/constants"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/tx"
)

type validateMsgParams struct {
	evmParams evmtypes.Params
	from      sdktypes.AccAddress
	ethTx     *ethtypes.Transaction
}

func (s *EvmUnitAnteTestSuite) TestValidateMsg() {
	keyring := testkeyring.New(2)

	testCases := []struct {
		name              string
		expectedError     error
		getFunctionParams func() validateMsgParams
	}{
		{
			name:          "fail: invalid from address, should be nil",
			expectedError: errortypes.ErrInvalidRequest,
			getFunctionParams: func() validateMsgParams {
				return validateMsgParams{
					evmParams: evmtypes.DefaultParams(),
					ethTx:     nil,
					from:      keyring.GetAccAddr(0),
				}
			},
		},
		{
			name:          "success: transfer with default params",
			expectedError: nil,
			getFunctionParams: func() validateMsgParams {
				txArgs := getTxByType("transfer", keyring.GetAddr(1))
				ethTx := txArgs.ToTx()
				return validateMsgParams{
					evmParams: evmtypes.DefaultParams(),
					ethTx:     ethTx,
					from:      nil,
				}
			},
		},
		{
			name:          "success: transfer with disable call and create",
			expectedError: evmtypes.ErrCallDisabled,
			getFunctionParams: func() validateMsgParams {
				txArgs := getTxByType("transfer", keyring.GetAddr(1))
				ethTx := txArgs.ToTx()
				params := evmtypes.DefaultParams()
				params.AccessControl.Call.AccessType = evmtypes.AccessTypeRestricted
				params.AccessControl.Create.AccessType = evmtypes.AccessTypeRestricted

				return validateMsgParams{
					evmParams: params,
					ethTx:     ethTx,
					from:      nil,
				}
			},
		},
		{
			name:          "success: call with default params",
			expectedError: nil,
			getFunctionParams: func() validateMsgParams {
				txArgs := getTxByType("call", keyring.GetAddr(1))
				ethTx := txArgs.ToTx()
				return validateMsgParams{
					evmParams: evmtypes.DefaultParams(),
					ethTx:     ethTx,
					from:      nil,
				}
			},
		},
		{
			name:          "success: call tx with disabled create",
			expectedError: nil,
			getFunctionParams: func() validateMsgParams {
				txArgs := getTxByType("call", keyring.GetAddr(1))
				ethTx := txArgs.ToTx()

				params := evmtypes.DefaultParams()
				params.AccessControl.Create.AccessType = evmtypes.AccessTypeRestricted

				return validateMsgParams{
					evmParams: params,
					ethTx:     ethTx,
					from:      nil,
				}
			},
		},
		{
			name:          "fail: call tx with disabled call",
			expectedError: evmtypes.ErrCallDisabled,
			getFunctionParams: func() validateMsgParams {
				txArgs := getTxByType("call", keyring.GetAddr(1))
				ethTx := txArgs.ToTx()

				params := evmtypes.DefaultParams()
				params.AccessControl.Call.AccessType = evmtypes.AccessTypeRestricted

				return validateMsgParams{
					evmParams: params,
					ethTx:     ethTx,
					from:      nil,
				}
			},
		},
		{
			name:          "success: create with default params",
			expectedError: nil,
			getFunctionParams: func() validateMsgParams {
				txArgs := getTxByType("create", keyring.GetAddr(1))
				ethTx := txArgs.ToTx()
				return validateMsgParams{
					evmParams: evmtypes.DefaultParams(),
					ethTx:     ethTx,
					from:      nil,
				}
			},
		},
		{
			name:          "success: create with disable call",
			expectedError: nil,
			getFunctionParams: func() validateMsgParams {
				txArgs := getTxByType("create", keyring.GetAddr(1))
				ethTx := txArgs.ToTx()

				params := evmtypes.DefaultParams()
				params.AccessControl.Call.AccessType = evmtypes.AccessTypeRestricted

				return validateMsgParams{
					evmParams: params,
					ethTx:     ethTx,
					from:      nil,
				}
			},
		},
		{
			name:          "fail: create with disable create",
			expectedError: evmtypes.ErrCreateDisabled,
			getFunctionParams: func() validateMsgParams {
				txArgs := getTxByType("create", keyring.GetAddr(1))
				ethTx := txArgs.ToTx()

				params := evmtypes.DefaultParams()
				params.AccessControl.Create.AccessType = evmtypes.AccessTypeRestricted

				return validateMsgParams{
					evmParams: params,
					ethTx:     ethTx,
					from:      nil,
				}
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			params := tc.getFunctionParams()

			// Function under test
			err := evm.ValidateMsg(
				params.evmParams,
				params.ethTx,
			)

			if tc.expectedError != nil {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.expectedError.Error())
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

func getTxByType(typeTx string, recipient common.Address) evmtypes.EvmTxArgs {
	switch typeTx {
	case "call":
		return evmtypes.EvmTxArgs{
			To:    &recipient,
			Input: []byte("call bytes"),
		}
	case "create":
		return evmtypes.EvmTxArgs{
			Input: []byte("create bytes"),
		}
	case "transfer":
		return evmtypes.EvmTxArgs{
			To:     &recipient,
			Amount: big.NewInt(100),
		}
	default:
		panic("invalid type")
	}
}

func (s *EvmUnitAnteTestSuite) TestCheckTxFee() {
	// amount represents 1 token in the 18 decimals representation.
	amount := math.NewInt(1e18)
	gasLimit := uint64(1e6)

	testCases := []struct {
		name       string
		txFee      *big.Int
		txGasLimit uint64
		expError   error
	}{
		{
			name:       "pass",
			txFee:      big.NewInt(amount.Int64()),
			txGasLimit: gasLimit,
			expError:   nil,
		},
		{
			name:       "fail: not enough tx fees",
			txFee:      big.NewInt(amount.Int64() - 1),
			txGasLimit: gasLimit,
			expError:   errortypes.ErrInvalidRequest,
		},
	}

	for _, chainID := range []testconstants.ChainID{
		testconstants.ExampleChainID,
		testconstants.SixDecimalsChainID,
	} {
		for _, tc := range testCases {
			s.Run(fmt.Sprintf("%s, %s", chainID.ChainID, tc.name), func() {
				// Call the configurator to set the EVM coin required for the
				// function to be tested.
				configurator := evmtypes.NewEVMConfigurator()
				configurator.ResetTestConfig()
				s.Require().NoError(configurator.WithEVMCoinInfo(testconstants.ExampleChainCoinInfo[chainID]).Configure())

				// If decimals is not 18 decimals, we have to convert txFeeInfo to original
				// decimals representation.
				evmExtendedDenom := evmtypes.GetEVMCoinExtendedDenom()

				coins := sdktypes.Coins{sdktypes.Coin{Denom: evmExtendedDenom, Amount: amount}}

				// This struct should hold values in the original representation
				txFeeInfo := &tx.Fee{
					Amount:   coins,
					GasLimit: gasLimit,
				}

				// Function under test
				err := evm.CheckTxFee(txFeeInfo, tc.txFee, tc.txGasLimit)

				if tc.expError != nil {
					s.Require().Error(err)
					s.Contains(err.Error(), tc.expError.Error())
				} else {
					s.Require().NoError(err)
				}
			})
		}
	}
}
