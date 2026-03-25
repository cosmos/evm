package filters

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestInvalidBlockRange(t *testing.T) {
	invalidCriteria := filters.FilterCriteria{
		FromBlock: big.NewInt(0x20),
		ToBlock:   big.NewInt(0x10),
	}
	t.Run("NewFilter", func(t *testing.T) {
		api := &PublicFilterAPI{
			filters: make(map[rpc.ID]*filter),
		}
		id, err := api.NewFilter(invalidCriteria)
		require.Equal(t, rpc.ID(""), id)
		require.Error(t, err)
		require.Len(t, api.filters, 0)
		require.ErrorIs(t, err, errInvalidBlockRange)
	})

	t.Run("GetFilterLogs", func(t *testing.T) {
		id := rpc.NewID()
		api := &PublicFilterAPI{
			filters: map[rpc.ID]*filter{
				id: {
					typ:  filters.LogsSubscription,
					crit: invalidCriteria,
				},
			},
		}
		logs, err := api.GetFilterLogs(context.Background(), id)
		require.Nil(t, logs)
		require.ErrorIs(t, err, errInvalidBlockRange)
	})

	t.Run("GetLogs", func(t *testing.T) {
		api := &PublicFilterAPI{}
		logs, err := api.GetLogs(context.Background(), invalidCriteria)
		require.Nil(t, logs)
		require.ErrorIs(t, err, errInvalidBlockRange)
	})
}

func TestTimeoutLoop_PanicOnNilCancel(t *testing.T) {
	api := &PublicFilterAPI{
		filters:   make(map[rpc.ID]*filter),
		filtersMu: sync.Mutex{},
		deadline:  10 * time.Millisecond,
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
