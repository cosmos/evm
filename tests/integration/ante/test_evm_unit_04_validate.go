package ante

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	protov2 "google.golang.org/protobuf/proto"

	"github.com/cosmos/evm/ante/evm"
	testconstants "github.com/cosmos/evm/testutil/constants"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
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

// mockTx is a mock transaction that implements sdktypes.Tx and ProtoTxProvider
type mockTx struct {
	protoTx *tx.Tx
}

func (m *mockTx) GetMsgs() []sdktypes.Msg {
	return nil
}

func (m *mockTx) GetMsgsV2() ([]protov2.Message, error) {
	return nil, nil
}

func (m *mockTx) ValidateBasic() error {
	return nil
}

func (m *mockTx) GetProtoTx() *tx.Tx {
	return m.protoTx
}

func (s *EvmUnitAnteTestSuite) TestValidateTx() {
	keyring := testkeyring.New(2)
	to := keyring.GetAddr(1)

	// Create a valid MsgEthereumTx
	validTxArgs := evmtypes.EvmTxArgs{
		To:     &to,
		Amount: big.NewInt(100),
	}
	msgEthTx := evmtypes.NewTx(&validTxArgs)
	msgEthTx.From = keyring.GetAddr(0).Bytes()

	// Create a valid proto transaction structure
	createValidProtoTx := func() *tx.Tx {
		option, err := codectypes.NewAnyWithValue(&evmtypes.ExtensionOptionsEthereumTx{})
		s.Require().NoError(err)

		evmExtendedDenom := evmtypes.GetEVMCoinExtendedDenom()
		feeAmount := sdktypes.NewCoins(sdktypes.NewCoin(evmExtendedDenom, math.NewInt(1000)))

		return &tx.Tx{
			Body: &tx.TxBody{
				Messages:                    []*codectypes.Any{},
				Memo:                        "",
				TimeoutHeight:               0,
				ExtensionOptions:            []*codectypes.Any{option},
				NonCriticalExtensionOptions: []*codectypes.Any{},
			},
			AuthInfo: &tx.AuthInfo{
				SignerInfos: []*tx.SignerInfo{},
				Fee: &tx.Fee{
					Amount:   feeAmount,
					GasLimit: 100000,
					Payer:    "",
					Granter:  "",
				},
				Tip: nil,
			},
			Signatures: [][]byte{},
		}
	}

	testCases := []struct {
		name        string
		createTx    func() sdktypes.Tx
		expectedErr error
		errContains string
	}{
		{
			name: "success: valid transaction",
			createTx: func() sdktypes.Tx {
				protoTx := createValidProtoTx()
				// Set the message
				msgAny, err := codectypes.NewAnyWithValue(msgEthTx)
				s.Require().NoError(err)
				protoTx.Body.Messages = []*codectypes.Any{msgAny}
				return &mockTx{protoTx: protoTx}
			},
			expectedErr: nil,
		},
		{
			name: "fail: transaction does not implement ProtoTxProvider",
			createTx: func() sdktypes.Tx {
				// Create a transaction that doesn't implement ProtoTxProvider
				return &mockTxWithoutProto{}
			},
			expectedErr: errortypes.ErrUnknownRequest,
			errContains: "didn't implement interface ProtoTxProvider",
		},
		{
			name: "fail: body Memo is not empty",
			createTx: func() sdktypes.Tx {
				protoTx := createValidProtoTx()
				protoTx.Body.Memo = "test memo"
				msgAny, err := codectypes.NewAnyWithValue(msgEthTx)
				s.Require().NoError(err)
				protoTx.Body.Messages = []*codectypes.Any{msgAny}
				return &mockTx{protoTx: protoTx}
			},
			expectedErr: errortypes.ErrInvalidRequest,
			errContains: "Memo",
		},
		{
			name: "fail: body TimeoutHeight is not zero",
			createTx: func() sdktypes.Tx {
				protoTx := createValidProtoTx()
				protoTx.Body.TimeoutHeight = 100
				msgAny, err := codectypes.NewAnyWithValue(msgEthTx)
				s.Require().NoError(err)
				protoTx.Body.Messages = []*codectypes.Any{msgAny}
				return &mockTx{protoTx: protoTx}
			},
			expectedErr: errortypes.ErrInvalidRequest,
			errContains: "TimeoutHeight",
		},
		{
			name: "fail: body NonCriticalExtensionOptions is not empty",
			createTx: func() sdktypes.Tx {
				protoTx := createValidProtoTx()
				option, err := codectypes.NewAnyWithValue(&evmtypes.ExtensionOptionsEthereumTx{})
				s.Require().NoError(err)
				protoTx.Body.NonCriticalExtensionOptions = []*codectypes.Any{option}
				msgAny, err := codectypes.NewAnyWithValue(msgEthTx)
				s.Require().NoError(err)
				protoTx.Body.Messages = []*codectypes.Any{msgAny}
				return &mockTx{protoTx: protoTx}
			},
			expectedErr: errortypes.ErrInvalidRequest,
			errContains: "NonCriticalExtensionOptions",
		},
		{
			name: "fail: body ExtensionOptions length is not 1",
			createTx: func() sdktypes.Tx {
				protoTx := createValidProtoTx()
				protoTx.Body.ExtensionOptions = []*codectypes.Any{} // empty
				msgAny, err := codectypes.NewAnyWithValue(msgEthTx)
				s.Require().NoError(err)
				protoTx.Body.Messages = []*codectypes.Any{msgAny}
				return &mockTx{protoTx: protoTx}
			},
			expectedErr: errortypes.ErrInvalidRequest,
			errContains: "ExtensionOptions should be 1",
		},
		{
			name: "fail: body ExtensionOptions length is greater than 1",
			createTx: func() sdktypes.Tx {
				protoTx := createValidProtoTx()
				option1, err := codectypes.NewAnyWithValue(&evmtypes.ExtensionOptionsEthereumTx{})
				s.Require().NoError(err)
				option2, err := codectypes.NewAnyWithValue(&evmtypes.ExtensionOptionsEthereumTx{})
				s.Require().NoError(err)
				protoTx.Body.ExtensionOptions = []*codectypes.Any{option1, option2}
				msgAny, err := codectypes.NewAnyWithValue(msgEthTx)
				s.Require().NoError(err)
				protoTx.Body.Messages = []*codectypes.Any{msgAny}
				return &mockTx{protoTx: protoTx}
			},
			expectedErr: errortypes.ErrInvalidRequest,
			errContains: "ExtensionOptions should be 1",
		},
		{
			name: "fail: AuthInfo SignerInfos is not empty",
			createTx: func() sdktypes.Tx {
				protoTx := createValidProtoTx()
				protoTx.AuthInfo.SignerInfos = []*tx.SignerInfo{
					{},
				}
				msgAny, err := codectypes.NewAnyWithValue(msgEthTx)
				s.Require().NoError(err)
				protoTx.Body.Messages = []*codectypes.Any{msgAny}
				return &mockTx{protoTx: protoTx}
			},
			expectedErr: errortypes.ErrInvalidRequest,
			errContains: "SignerInfos should be empty",
		},
		{
			name: "fail: AuthInfo Fee is nil",
			createTx: func() sdktypes.Tx {
				protoTx := createValidProtoTx()
				protoTx.AuthInfo.Fee = nil
				msgAny, err := codectypes.NewAnyWithValue(msgEthTx)
				s.Require().NoError(err)
				protoTx.Body.Messages = []*codectypes.Any{msgAny}
				return &mockTx{protoTx: protoTx}
			},
			expectedErr: errortypes.ErrInvalidRequest,
			errContains: "authInfo.Fee should not be nil",
		},
		{
			name: "fail: AuthInfo Fee Payer is not empty",
			createTx: func() sdktypes.Tx {
				protoTx := createValidProtoTx()
				protoTx.AuthInfo.Fee.Payer = "cosmos1test"
				msgAny, err := codectypes.NewAnyWithValue(msgEthTx)
				s.Require().NoError(err)
				protoTx.Body.Messages = []*codectypes.Any{msgAny}
				return &mockTx{protoTx: protoTx}
			},
			expectedErr: errortypes.ErrInvalidRequest,
			errContains: "payer and granter should be empty",
		},
		{
			name: "fail: AuthInfo Fee Granter is not empty",
			createTx: func() sdktypes.Tx {
				protoTx := createValidProtoTx()
				protoTx.AuthInfo.Fee.Granter = "cosmos1test"
				msgAny, err := codectypes.NewAnyWithValue(msgEthTx)
				s.Require().NoError(err)
				protoTx.Body.Messages = []*codectypes.Any{msgAny}
				return &mockTx{protoTx: protoTx}
			},
			expectedErr: errortypes.ErrInvalidRequest,
			errContains: "payer and granter should be empty",
		},
		{
			name: "fail: AuthInfo Tip is not nil",
			createTx: func() sdktypes.Tx {
				protoTx := createValidProtoTx()
				protoTx.AuthInfo.Tip = &tx.Tip{
					Amount: sdktypes.NewCoins(),
				}
				msgAny, err := codectypes.NewAnyWithValue(msgEthTx)
				s.Require().NoError(err)
				protoTx.Body.Messages = []*codectypes.Any{msgAny}
				return &mockTx{protoTx: protoTx}
			},
			expectedErr: errortypes.ErrInvalidRequest,
			errContains: "authInfo.Tip must be nil",
		},
		{
			name: "fail: Signatures is not empty",
			createTx: func() sdktypes.Tx {
				protoTx := createValidProtoTx()
				protoTx.Signatures = [][]byte{[]byte("signature")}
				msgAny, err := codectypes.NewAnyWithValue(msgEthTx)
				s.Require().NoError(err)
				protoTx.Body.Messages = []*codectypes.Any{msgAny}
				return &mockTx{protoTx: protoTx}
			},
			expectedErr: errortypes.ErrInvalidRequest,
			errContains: "Signatures should be empty",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			testTx := tc.createTx()

			// Function under test
			fee, err := evm.ValidateTx(testTx)

			if tc.expectedErr != nil {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.expectedErr.Error())
				if tc.errContains != "" {
					s.Contains(err.Error(), tc.errContains)
				}
				s.Nil(fee)
			} else {
				s.Require().NoError(err)
				s.NotNil(fee)
			}
		})
	}
}

// mockTxWithoutProto is a mock transaction that doesn't implement ProtoTxProvider
type mockTxWithoutProto struct{}

func (m *mockTxWithoutProto) GetMsgs() []sdktypes.Msg {
	return nil
}

func (m *mockTxWithoutProto) GetMsgsV2() ([]protov2.Message, error) {
	return nil, nil
}

func (m *mockTxWithoutProto) ValidateBasic() error {
	return nil
}
