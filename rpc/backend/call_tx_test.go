package backend

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	tmbytes "github.com/cometbft/cometbft/libs/bytes"
	tmrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/evm/rpc/backend/mocks"
	"github.com/cosmos/evm/testutil/constants"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"
)

func TestSetTxDefaults(t *testing.T) {
	from := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	to := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdef")
	latestHeight := int64(10)
	estimatedGas := hexutil.Uint64(21000)
	providedGas := hexutil.Uint64(50000)
	nonce := hexutil.Uint64(0)
	gasPrice := (*hexutil.Big)(common.Big1)
	maxFeePerGas := (*hexutil.Big)(common.Big2)

	configurator := evmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	require.NoError(t, evmtypes.SetChainConfig(evmtypes.DefaultChainConfig(constants.ExampleChainID.EVMChainID)))
	require.NoError(t, configurator.WithEVMCoinInfo(constants.ExampleChainCoinInfo[constants.ExampleChainID]).Configure())

	testCases := []struct {
		name              string
		malleate          func() evmtypes.TransactionArgs
		expectError       bool
		errorContains     string
		expectGas         *hexutil.Uint64
		expectEstimateGas bool
	}{
		{
			name: "estimate gas with latest block",
			malleate: func() evmtypes.TransactionArgs {
				return evmtypes.TransactionArgs{
					From:  &from,
					To:    &to,
					Nonce: &nonce,
				}
			},
			expectGas:         &estimatedGas,
			expectEstimateGas: true,
		},
		{
			name: "preserve provided gas",
			malleate: func() evmtypes.TransactionArgs {
				return evmtypes.TransactionArgs{
					From:  &from,
					To:    &to,
					Gas:   &providedGas,
					Nonce: &nonce,
				}
			},
			expectGas: &providedGas,
		},
		{
			name: "error when gas price and dynamic fee are both set",
			malleate: func() evmtypes.TransactionArgs {
				return evmtypes.TransactionArgs{
					From:         &from,
					To:           &to,
					GasPrice:     gasPrice,
					MaxFeePerGas: maxFeePerGas,
				}
			},
			expectError:   true,
			errorContains: "both gasPrice and (maxFeePerGas or maxPriorityFeePerGas) specified",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backend := setupMockBackend(t)
			mockClient := backend.ClientCtx.Client.(*mocks.Client)
			mockEVMQueryClient := backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
			mockFeeMarketQueryClient := backend.QueryClient.FeeMarket.(*mocks.FeeMarketQueryClient)
			mockEVMQueryClient.ExpectedCalls = nil

			mockEVMQueryClient.On("Params", mock.Anything, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					header := args.Get(2).(grpc.HeaderCallOption).HeaderAddr
					*header = metadata.MD{}
					header.Set(grpctypes.GRPCBlockHeightHeader, "10")
				}).
				Return(&evmtypes.QueryParamsResponse{Params: evmtypes.DefaultParams()}, nil).
				Maybe()

			mockClient.On("Block", mock.Anything, &latestHeight).Return(&tmrpctypes.ResultBlock{
				BlockID: tmtypes.BlockID{Hash: tmbytes.HexBytes{0x01}},
				Block: &tmtypes.Block{
					Header: tmtypes.Header{
						Height:  latestHeight,
						Time:    time.Now(),
						ChainID: constants.ExampleChainID.ChainID,
					},
				},
			}, nil).Maybe()
			mockClient.On("BlockResults", mock.Anything, &latestHeight).Return(&tmrpctypes.ResultBlockResults{
				Height: latestHeight,
			}, nil).Maybe()
			mockClient.On("ConsensusParams", mock.Anything, &latestHeight).Return(&tmrpctypes.ResultConsensusParams{
				ConsensusParams: tmtypes.ConsensusParams{Block: tmtypes.BlockParams{MaxGas: 10000000}},
			}, nil).Maybe()
			mockClient.On("Header", mock.Anything, &latestHeight).Return(&tmrpctypes.ResultHeader{
				Header: &tmtypes.Header{
					Height:          latestHeight,
					Time:            time.Now(),
					ChainID:         constants.ExampleChainID.ChainID,
					ProposerAddress: []byte{0x01},
				},
			}, nil).Maybe()
			mockEVMQueryClient.On("BaseFee", mock.Anything, mock.Anything).
				Return(&evmtypes.QueryBaseFeeResponse{}, nil).Maybe()
			mockEVMQueryClient.On("ValidatorAccount", mock.Anything, mock.Anything).
				Return(&evmtypes.QueryValidatorAccountResponse{}, assert.AnError).Maybe()
			mockFeeMarketQueryClient.On("Params", mock.Anything, mock.Anything).
				Return(&feemarkettypes.QueryParamsResponse{Params: feemarkettypes.DefaultParams()}, nil).Maybe()

			if tc.expectEstimateGas {
				mockEVMQueryClient.On("EstimateGas", mock.Anything, mock.AnythingOfType("*types.EthCallRequest")).
					Return(&evmtypes.EstimateGasResponse{Gas: uint64(estimatedGas)}, nil).Once()
			}

			result, err := backend.SetTxDefaults(context.Background(), tc.malleate())
			if tc.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorContains)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result.Gas)
			require.Equal(t, *tc.expectGas, *result.Gas)
		})
	}
}
