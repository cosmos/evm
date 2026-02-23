package filters

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"

	filtermocks "github.com/cosmos/evm/rpc/namespaces/ethereum/eth/filters/mocks"
	"github.com/cosmos/evm/rpc/stream"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/client"
)

func TestTimeoutLoop_PanicOnNilCancel(t *testing.T) {
	api := &PublicFilterAPI{
		filters:         make(map[rpc.ID]*filter),
		filtersMu:       sync.Mutex{},
		deadline:        10 * time.Millisecond,
		cleanupInterval: 10 * time.Millisecond,
	}
	api.filters[rpc.NewID()] = &filter{
		typ:      filters.BlocksSubscription,
		deadline: time.NewTimer(0),
	}
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("cancel panic")
			}
			close(done)
		}()
		api.timeoutLoop()
	}()
	panicked := false
	select {
	case <-done:
		panicked = true
	case <-time.After(100 * time.Millisecond):
	}
	require.False(t, panicked)
}

func TestNewBlockFilter_DisabledReturnsError(t *testing.T) {
	backend := filtermocks.NewBackend(t)
	backend.EXPECT().RPCFilterCap().Return(int32(0)).Once()

	api := NewPublicAPIWithDeadline(
		log.NewNopLogger(),
		client.Context{},
		stream.NewRPCStreams(nil, log.NewNopLogger(), nil),
		backend,
		time.Minute,
	)

	id, err := api.NewBlockFilter(context.Background())
	require.Error(t, err)
	require.Empty(t, id)
	require.Contains(t, err.Error(), "filter creation is disabled")
}

func TestNewPendingTransactionFilter_PerClientCap(t *testing.T) {
	backend := filtermocks.NewBackend(t)
	backend.EXPECT().RPCFilterCap().Return(int32(10)).Times(3)

	api := NewPublicAPIWithDeadline(
		log.NewNopLogger(),
		client.Context{},
		stream.NewRPCStreams(nil, log.NewNopLogger(), nil),
		backend,
		time.Minute,
	)
	api.clientCap = 1

	id1, err := api.NewPendingTransactionFilter(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, id1)

	id2, err := api.NewPendingTransactionFilter(context.Background())
	require.Error(t, err)
	require.Empty(t, id2)
	require.Contains(t, err.Error(), "per-client max limit reached")

	removed := api.UninstallFilter(id1)
	require.True(t, removed)

	id3, err := api.NewPendingTransactionFilter(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, id3)
}

func TestNewFilter_GlobalCapReturnsError(t *testing.T) {
	backend := filtermocks.NewBackend(t)
	backend.EXPECT().RPCFilterCap().Return(int32(0)).Once()

	api := NewPublicAPIWithDeadline(
		log.NewNopLogger(),
		client.Context{},
		stream.NewRPCStreams(nil, log.NewNopLogger(), nil),
		backend,
		time.Minute,
	)

	id, err := api.NewFilter(context.Background(), filters.FilterCriteria{})
	require.Error(t, err)
	require.Empty(t, id)
	require.Contains(t, err.Error(), "filter creation is disabled")
}
