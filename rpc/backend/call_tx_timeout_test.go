package backend

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	cmtrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/evm/rpc/backend/mocks"
	rpctypes "github.com/cosmos/evm/rpc/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func TestEstimateGasAppliesEVMTimeout(t *testing.T) {
	backend := setupMockBackend(t)
	backend.Cfg.JSONRPC.EVMTimeout = 10 * time.Millisecond

	client := backend.ClientCtx.Client.(*mocks.Client)
	client.On("Header", mock.Anything, mock.Anything).Return(&cmtrpctypes.ResultHeader{
		Header: &tmtypes.Header{Height: 1},
	}, nil)

	queryClient := backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
	queryClient.On("EstimateGas", mock.Anything, mock.Anything).Return(
		func(ctx context.Context, _ *evmtypes.EthCallRequest, _ ...grpc.CallOption) (*evmtypes.EstimateGasResponse, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
		nil,
	)

	from := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	args := evmtypes.TransactionArgs{From: &from}
	blockNum := rpctypes.BlockNumber(1)
	_, err := backend.EstimateGas(context.Background(), args, &rpctypes.BlockNumberOrHash{BlockNumber: &blockNum}, nil)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestDoCallAppliesEVMTimeout(t *testing.T) {
	backend := setupMockBackend(t)
	backend.Cfg.JSONRPC.EVMTimeout = 10 * time.Millisecond

	client := backend.ClientCtx.Client.(*mocks.Client)
	client.On("Header", mock.Anything, mock.Anything).Return(&cmtrpctypes.ResultHeader{
		Header: &tmtypes.Header{Height: 1},
	}, nil)

	queryClient := backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
	queryClient.On("EthCall", mock.Anything, mock.Anything).Return(
		func(ctx context.Context, _ *evmtypes.EthCallRequest, _ ...grpc.CallOption) (*evmtypes.MsgEthereumTxResponse, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
		nil,
	)

	from := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	args := evmtypes.TransactionArgs{From: &from}
	_, err := backend.DoCall(context.Background(), args, rpctypes.BlockNumber(1), nil)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestEstimateGasZeroTimeoutHasNoDeadline(t *testing.T) {
	backend := setupMockBackend(t)
	backend.Cfg.JSONRPC.EVMTimeout = 0

	client := backend.ClientCtx.Client.(*mocks.Client)
	client.On("Header", mock.Anything, mock.Anything).Return(&cmtrpctypes.ResultHeader{
		Header: &tmtypes.Header{Height: 1},
	}, nil)

	queryClient := backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
	queryClient.On("EstimateGas", mock.Anything, mock.Anything).Return(
		func(ctx context.Context, _ *evmtypes.EthCallRequest, _ ...grpc.CallOption) (*evmtypes.EstimateGasResponse, error) {
			require.NoError(t, ctx.Err())
			_, ok := ctx.Deadline()
			require.False(t, ok)
			return &evmtypes.EstimateGasResponse{Gas: 21000}, nil
		},
		nil,
	)

	from := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	args := evmtypes.TransactionArgs{From: &from}
	blockNum := rpctypes.BlockNumber(1)
	result, err := backend.EstimateGas(context.Background(), args, &rpctypes.BlockNumberOrHash{BlockNumber: &blockNum}, nil)
	require.NoError(t, err)
	require.Equal(t, hexutil.Uint64(21000), result)
}
