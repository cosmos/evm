package filters

import (
	"net/http/httptest"
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

func newFilterAPITestSubject(t *testing.T, backend Backend) *PublicFilterAPI {
	t.Helper()
	api := NewPublicAPIWithDeadline(
		log.NewNopLogger(),
		client.Context{},
		stream.NewRPCStreams(nil, log.NewNopLogger(), nil),
		backend,
		time.Minute,
	)
	t.Cleanup(api.Stop)
	return api
}

func newFilterAPITestSubjectWithOptions(t *testing.T, backend Backend, deadline, cleanupInterval time.Duration) *PublicFilterAPI {
	t.Helper()
	api := newPublicAPIWithOptions(
		log.NewNopLogger(),
		client.Context{},
		stream.NewRPCStreams(nil, log.NewNopLogger(), nil),
		backend,
		deadline,
		cleanupInterval,
	)
	t.Cleanup(api.Stop)
	return api
}

func newHTTPRPCClientForFilterAPI(t *testing.T, api *PublicFilterAPI) *rpc.Client {
	t.Helper()

	rpcSrv := rpc.NewServer()
	err := rpcSrv.RegisterName("eth", api)
	require.NoError(t, err)
	t.Cleanup(rpcSrv.Stop)

	ts := httptest.NewServer(rpcSrv)
	t.Cleanup(ts.Close)

	rpcClient, err := rpc.Dial(ts.URL)
	require.NoError(t, err)
	t.Cleanup(rpcClient.Close)

	return rpcClient
}

func requireNewPendingTxFilterSuccess(t *testing.T, rpcClient *rpc.Client) rpc.ID {
	t.Helper()
	var id rpc.ID
	require.NoError(t, rpcClient.Call(&id, "eth_newPendingTransactionFilter"))
	require.NotEmpty(t, id)
	return id
}

func TestTimeoutLoop_StopHalts(t *testing.T) {
	api := &PublicFilterAPI{
		filters:         make(map[rpc.ID]*filter),
		filtersMu:       sync.Mutex{},
		deadline:        10 * time.Millisecond,
		cleanupInterval: 10 * time.Millisecond,
		stop:            make(chan struct{}),
	}
	api.filters[rpc.NewID()] = &filter{
		typ:      filters.BlocksSubscription,
		deadline: time.NewTimer(0),
	}
	done := make(chan struct{})
	go func() {
		api.timeoutLoop()
		close(done)
	}()
	api.Stop()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timeoutLoop did not exit after Stop")
	}
}

func TestNewPendingTransactionFilter_HTTPContext(t *testing.T) {
	backend := filtermocks.NewBackend(t)

	api := newFilterAPITestSubject(t, backend)
	rpcClient := newHTTPRPCClientForFilterAPI(t, api)
	id := requireNewPendingTxFilterSuccess(t, rpcClient)
	require.NotEmpty(t, id)
}

func TestNewPendingTransactionFilter_NoCapExhaustion(t *testing.T) {
	backend := filtermocks.NewBackend(t)

	api := newFilterAPITestSubject(t, backend)
	rpcClient := newHTTPRPCClientForFilterAPI(t, api)

	// Create more filters than the old default cap (200) to confirm the
	// node no longer refuses filter creation — matches upstream geth,
	// which relies on the idle-deadline sweep rather than a fixed cap.
	for i := 0; i < 300; i++ {
		requireNewPendingTxFilterSuccess(t, rpcClient)
	}
	require.Equal(t, 300, func() int {
		api.filtersMu.Lock()
		defer api.filtersMu.Unlock()
		return len(api.filters)
	}())
}

func TestFilter_ExpiresAfterDeadline(t *testing.T) {
	backend := filtermocks.NewBackend(t)

	api := newFilterAPITestSubjectWithOptions(t, backend, 20*time.Millisecond, 5*time.Millisecond)
	rpcClient := newHTTPRPCClientForFilterAPI(t, api)
	requireNewPendingTxFilterSuccess(t, rpcClient)

	require.Eventually(t, func() bool {
		api.filtersMu.Lock()
		defer api.filtersMu.Unlock()
		return len(api.filters) == 0
	}, 400*time.Millisecond, 10*time.Millisecond)
}

func TestDeleteFilterLocked_RemovesAndReportsMissing(t *testing.T) {
	api := &PublicFilterAPI{
		filters: make(map[rpc.ID]*filter),
	}

	id := rpc.NewID()
	api.filters[id] = &filter{deadline: time.NewTimer(time.Minute)}

	require.True(t, api.deleteFilterLocked(id))
	_, exists := api.filters[id]
	require.False(t, exists)

	require.False(t, api.deleteFilterLocked(rpc.NewID()))
}
