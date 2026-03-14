package filters

import (
	"context"
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

const (
	dummyHTTPClientA = "http:203.0.113.10"
	dummyHTTPClientB = "http:203.0.113.11"
	dummyWSClientA   = "ws:198.51.100.25"
)

func newFilterAPITestSubject(backend Backend) *PublicFilterAPI {
	return NewPublicAPIWithDeadline(
		log.NewNopLogger(),
		client.Context{},
		stream.NewRPCStreams(nil, log.NewNopLogger(), nil),
		backend,
		time.Minute,
	)
}

func newFilterAPITestSubjectWithOptions(backend Backend, deadline, cleanupInterval time.Duration, clientCap int32) *PublicFilterAPI {
	return newPublicAPIWithOptions(
		log.NewNopLogger(),
		client.Context{},
		stream.NewRPCStreams(nil, log.NewNopLogger(), nil),
		backend,
		deadline,
		cleanupInterval,
		clientCap,
	)
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

func requireNewPendingTxFilterError(t *testing.T, rpcClient *rpc.Client, expectedErrContains string) {
	t.Helper()
	var id rpc.ID
	err := rpcClient.Call(&id, "eth_newPendingTransactionFilter")
	require.Error(t, err)
	require.Contains(t, err.Error(), expectedErrContains)
}

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

	api := newFilterAPITestSubject(backend)
	id, err := api.NewBlockFilter(context.Background())
	require.Error(t, err)
	require.Empty(t, id)
	require.Contains(t, err.Error(), "filter creation is disabled")
}

func TestNewPendingTransactionFilter_PerClientCap(t *testing.T) {
	backend := filtermocks.NewBackend(t)
	backend.EXPECT().RPCFilterCap().Return(int32(10)).Times(3)

	api := newFilterAPITestSubject(backend)
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

	api := newFilterAPITestSubject(backend)
	id, err := api.NewFilter(context.Background(), filters.FilterCriteria{})
	require.Error(t, err)
	require.Empty(t, id)
	require.Contains(t, err.Error(), "filter creation is disabled")
}

func TestEnsureFilterCreationAllowedLocked_PerClientIsolation(t *testing.T) {
	backend := filtermocks.NewBackend(t)
	backend.EXPECT().RPCFilterCap().Return(int32(10)).Times(2)

	api := newFilterAPITestSubject(backend)
	api.clientCap = 1

	id := api.installFilterLocked(dummyHTTPClientA, &filter{deadline: time.NewTimer(time.Minute)})
	require.NotEmpty(t, id)
	require.Equal(t, 1, api.clientFilterCount[dummyHTTPClientA])

	err := api.ensureFilterCreationAllowedLocked(dummyHTTPClientA)
	require.Error(t, err)
	require.Contains(t, err.Error(), "per-client max limit reached")

	err = api.ensureFilterCreationAllowedLocked(dummyHTTPClientB)
	require.NoError(t, err)
}

func TestEnsureFilterCreationAllowedLocked_GlobalCapPrecedence(t *testing.T) {
	backend := filtermocks.NewBackend(t)
	backend.EXPECT().RPCFilterCap().Return(int32(1)).Once()

	api := newFilterAPITestSubject(backend)
	api.clientCap = 10
	api.installFilterLocked(dummyHTTPClientA, &filter{deadline: time.NewTimer(time.Minute)})

	err := api.ensureFilterCreationAllowedLocked(dummyHTTPClientB)
	require.Error(t, err)
	require.Contains(t, err.Error(), "max limit reached")
}

func TestDeleteFilterLocked_DecrementsPerClientCounter(t *testing.T) {
	api := &PublicFilterAPI{
		filters:           make(map[rpc.ID]*filter),
		clientFilterCount: make(map[string]int),
	}

	idA := rpc.NewID()
	idB := rpc.NewID()
	idC := rpc.NewID()

	api.filters[idA] = &filter{owner: dummyHTTPClientA, deadline: time.NewTimer(time.Minute)}
	api.filters[idB] = &filter{owner: dummyHTTPClientA, deadline: time.NewTimer(time.Minute)}
	api.filters[idC] = &filter{owner: dummyWSClientA, deadline: time.NewTimer(time.Minute)}
	api.clientFilterCount[dummyHTTPClientA] = 2
	api.clientFilterCount[dummyWSClientA] = 1

	require.True(t, api.deleteFilterLocked(idA))
	require.Equal(t, 1, api.clientFilterCount[dummyHTTPClientA])

	require.True(t, api.deleteFilterLocked(idB))
	_, exists := api.clientFilterCount[dummyHTTPClientA]
	require.False(t, exists)

	require.True(t, api.deleteFilterLocked(idC))
	_, exists = api.clientFilterCount[dummyWSClientA]
	require.False(t, exists)

	require.False(t, api.deleteFilterLocked(rpc.NewID()))
}

func TestClientIPFromContext_DefaultsToUnknown(t *testing.T) {
	api := &PublicFilterAPI{}
	clientIP := api.clientIPFromContext(context.Background())
	require.Equal(t, "unknown", clientIP)
}

func TestNewPendingTransactionFilter_PerClientCap_HTTPContext(t *testing.T) {
	backend := filtermocks.NewBackend(t)
	backend.EXPECT().RPCFilterCap().Return(int32(10)).Times(3)

	api := newFilterAPITestSubject(backend)
	api.clientCap = 1
	rpcClient := newHTTPRPCClientForFilterAPI(t, api)
	id1 := requireNewPendingTxFilterSuccess(t, rpcClient)
	requireNewPendingTxFilterError(t, rpcClient, "per-client max limit reached")
	var removed bool
	require.NoError(t, rpcClient.Call(&removed, "eth_uninstallFilter", id1))
	require.True(t, removed)
	requireNewPendingTxFilterSuccess(t, rpcClient)
}

func TestNewPendingTransactionFilter_Disabled_HTTPContext(t *testing.T) {
	backend := filtermocks.NewBackend(t)
	backend.EXPECT().RPCFilterCap().Return(int32(0)).Once()

	api := newFilterAPITestSubject(backend)
	rpcClient := newHTTPRPCClientForFilterAPI(t, api)
	requireNewPendingTxFilterError(t, rpcClient, "filter creation is disabled")
}

func TestNewPendingTransactionFilter_GlobalCap_HTTPContext(t *testing.T) {
	backend := filtermocks.NewBackend(t)
	backend.EXPECT().RPCFilterCap().Return(int32(1)).Times(2)

	api := newFilterAPITestSubject(backend)
	api.clientCap = 10
	rpcClient := newHTTPRPCClientForFilterAPI(t, api)
	requireNewPendingTxFilterSuccess(t, rpcClient)
	requireNewPendingTxFilterError(t, rpcClient, "max limit reached")
}

func TestNewPendingTransactionFilter_PerClientCap_RecoversAfterTimeout_HTTPContext(t *testing.T) {
	backend := filtermocks.NewBackend(t)
	backend.EXPECT().RPCFilterCap().Return(int32(10)).Times(3)

	api := newFilterAPITestSubjectWithOptions(backend, 20*time.Millisecond, 5*time.Millisecond, 1)
	rpcClient := newHTTPRPCClientForFilterAPI(t, api)
	requireNewPendingTxFilterSuccess(t, rpcClient)
	requireNewPendingTxFilterError(t, rpcClient, "per-client max limit reached")

	require.Eventually(t, func() bool {
		api.filtersMu.Lock()
		defer api.filtersMu.Unlock()
		return len(api.filters) == 0 && len(api.clientFilterCount) == 0
	}, 400*time.Millisecond, 10*time.Millisecond)

	requireNewPendingTxFilterSuccess(t, rpcClient)
}
