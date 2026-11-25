package backend

import (
	"context"
	"errors"
	"math/big"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	tmrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/evm/rpc/backend/mocks"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"
)

// TestSetTxDefaults_NilFrom checks that SetTxDefaults fills the nonce for args
// without a "from" field instead of dereferencing the nil pointer, matching
// geth, which defaults the sender to the zero address.
func TestSetTxDefaults_NilFrom(t *testing.T) {
	backend := setupMockBackend(t)
	height := int64(1)

	// building the latest header reads the global chain config
	if evmtypes.GetChainConfig() == nil {
		require.NoError(t, evmtypes.SetChainConfig(nil))
	}

	// CurrentHeader resolves "latest" through the gRPC block height header
	mockEVMQueryClient := backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
	mockEVMQueryClient.ExpectedCalls = nil
	mockEVMQueryClient.On("Params", mock.Anything, mock.Anything, mock.Anything).
		Return(&evmtypes.QueryParamsResponse{Params: evmtypes.DefaultParams()}, nil).
		Run(func(args mock.Arguments) {
			for _, opt := range args[2:] {
				if h, ok := opt.(grpc.HeaderCallOption); ok {
					*h.HeaderAddr = metadata.Pairs(grpctypes.GRPCBlockHeightHeader, strconv.FormatInt(height, 10))
				}
			}
		})
	mockEVMQueryClient.On("BaseFee", mock.Anything, mock.Anything).Return(&evmtypes.QueryBaseFeeResponse{}, nil).Maybe()
	mockEVMQueryClient.On("ValidatorAccount", mock.Anything, mock.Anything).Return(nil, errors.New("not found")).Maybe()

	mockClient := backend.ClientCtx.Client.(*mocks.Client)
	mockClient.On("Block", mock.Anything, &height).
		Return(&tmrpctypes.ResultBlock{Block: &tmtypes.Block{Header: tmtypes.Header{Height: height}}}, nil)
	mockClient.On("BlockResults", mock.Anything, &height).
		Return(&tmrpctypes.ResultBlockResults{Height: height}, nil)
	mockClient.On("ConsensusParams", mock.Anything, mock.Anything).Return(nil, errors.New("not found")).Maybe()
	// the account query fails; SetTxDefaults ignores nonce lookup errors
	mockClient.On("ABCIQueryWithOptions", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("account query failed")).Maybe()

	to := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdef")
	gas := hexutil.Uint64(21000)
	args := evmtypes.TransactionArgs{
		To:       &to,
		Gas:      &gas,
		GasPrice: (*hexutil.Big)(big.NewInt(20000000000)),
	}

	var (
		res evmtypes.TransactionArgs
		err error
	)
	require.NotPanics(t, func() {
		res, err = backend.SetTxDefaults(context.Background(), args)
	})
	require.NoError(t, err)
	require.Nil(t, res.From)
	require.NotNil(t, res.Nonce)
	require.Equal(t, hexutil.Uint64(0), *res.Nonce)
}
