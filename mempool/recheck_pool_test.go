package mempool_test

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/mempool/internal/heightsync"
	"github.com/cosmos/evm/mempool/reserver"

	"cosmossdk.io/log/v2"
	sdkmath "cosmossdk.io/math"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
)

// ----------------------------------------------------------------------------
// Mock Rechecker
// ----------------------------------------------------------------------------

// mockRechecker wraps an sdk.AnteHandler to implement the Rechecker interface
// for unit tests.
type mockRechecker struct {
	ctx         sdk.Context
	anteHandler sdk.AnteHandler
}

func newMockRechecker(ctx sdk.Context, anteHandler sdk.AnteHandler) *mockRechecker {
	return &mockRechecker{
		ctx:         ctx,
		anteHandler: anteHandler,
	}
}

func (m *mockRechecker) GetContext() (sdk.Context, func()) {
	if m.ctx.MultiStore() == nil {
		return sdk.Context{}, func() {}
	}
	return m.ctx.CacheContext()
}

func (m *mockRechecker) RecheckCosmos(ctx sdk.Context, tx sdk.Tx) (sdk.Context, error) {
	return m.anteHandler(ctx, tx, false)
}

func (m *mockRechecker) Update(ctx sdk.Context, _ *ethtypes.Header) {
	m.ctx = ctx
}

// ----------------------------------------------------------------------------
// Mock ContextProvider
// ----------------------------------------------------------------------------

// mockContextProvider implements ContextProvider for unit tests.
type mockContextProvider struct {
	ctx          sdk.Context
	getContextFn func() (sdk.Context, error)
	ctxErr       error
}

func newMockContextProvider(ctx sdk.Context) *mockContextProvider {
	return &mockContextProvider{ctx: ctx}
}

func (m *mockContextProvider) GetLatestContext() (sdk.Context, error) {
	if m.ctxErr != nil {
		return sdk.Context{}, m.ctxErr
	}
	if m.getContextFn != nil {
		return m.getContextFn()
	}
	ctx, _ := m.ctx.CacheContext()
	return ctx, nil
}

func (m *mockContextProvider) CurrentBlock() *ethtypes.Header {
	return &ethtypes.Header{Number: big.NewInt(10)}
}

// testHeader creates a minimal header for testing with the given height.
func testHeader(height int64) *ethtypes.Header {
	return &ethtypes.Header{
		Number:   big.NewInt(height),
		GasLimit: 100_000_000,
	}
}

// ----------------------------------------------------------------------------
// Insert/Remove
// ----------------------------------------------------------------------------

func TestRecheckMempool_Insert(t *testing.T) {
	tests := []struct {
		name          string
		setupReserver func(*reserver.ReservationTracker, common.Address)
		anteErr       error
		poolInsertErr error
		expectErr     error
		expectCount   int
		expectHeld    bool
	}{
		{
			name:        "success",
			expectErr:   nil,
			expectCount: 1,
			expectHeld:  true,
		},
		{
			name: "address already reserved by another pool",
			setupReserver: func(tracker *reserver.ReservationTracker, addr common.Address) {
				otherHandle := tracker.NewHandle(999)
				require.NoError(t, otherHandle.Hold(addr))
			},
			expectErr:   reserver.ErrAlreadyReserved,
			expectCount: 0,
			expectHeld:  true, // still held by pool 999
		},
		{
			name:        "ante handler failure releases reservation",
			anteErr:     errors.New("insufficient funds"),
			expectErr:   errors.New("ante handler failed"),
			expectCount: 0,
			expectHeld:  false,
		},
		{
			name:          "pool insert failure releases reservation",
			poolInsertErr: errors.New("pool full"),
			expectErr:     errors.New("pool full"),
			expectCount:   0,
			expectHeld:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			acc := newRecheckTestAccount(t)
			tracker := reserver.NewReservationTracker()
			handle := tracker.NewHandle(1)

			if tc.setupReserver != nil {
				tc.setupReserver(tracker, acc.address)
			}

			pool := &recheckMockPool{insertErr: tc.poolInsertErr}

			anteHandler := func(ctx sdk.Context, tx sdk.Tx, _ bool) (sdk.Context, error) {
				if tc.anteErr != nil {
					return sdk.Context{}, tc.anteErr
				}
				return ctx, nil
			}

			ctx := newRecheckTestContext()
			bc := newMockContextProvider(ctx)
			rc := newMockRechecker(ctx, anteHandler)

			mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, newTestRecheckedTxs(), bc)

			tx := newRecheckTestTx(t, acc.key)
			err := mp.Insert(ctx, tx)

			if tc.expectErr != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErr.Error())
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.expectCount, mp.CountTx())

			// Verify reservation state using handle from a different pool
			otherHandle := tracker.NewHandle(2)
			if tc.expectHeld {
				require.True(t, otherHandle.Has(acc.address), "address should be reserved by some pool")
			} else {
				require.False(t, otherHandle.Has(acc.address), "address should not be reserved")
			}
		})
	}
}

func TestRecheckMempool_Remove(t *testing.T) {
	tests := []struct {
		name       string
		poolErr    error
		expectErr  bool
		expectHeld bool
	}{
		{
			name:       "success releases reservation",
			expectErr:  false,
			expectHeld: false,
		},
		{
			name:       "pool error does not release",
			poolErr:    errors.New("tx not found"),
			expectErr:  true,
			expectHeld: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			acc := newRecheckTestAccount(t)
			tracker := reserver.NewReservationTracker()
			handle := tracker.NewHandle(1)

			pool := &recheckMockPool{}
			ctx := newRecheckTestContext()
			bc := newMockContextProvider(ctx)
			rc := newMockRechecker(ctx, noopAnteHandler)

			mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, newTestRecheckedTxs(), bc)

			tx := newRecheckTestTx(t, acc.key)
			require.NoError(t, mp.Insert(ctx, tx))

			otherHandle := tracker.NewHandle(2)
			require.True(t, otherHandle.Has(acc.address), "address should be reserved after insert")

			if tc.poolErr != nil {
				pool.removeErr = tc.poolErr
			}

			err := mp.Remove(tx)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tc.expectHeld {
				require.True(t, otherHandle.Has(acc.address))
			} else {
				require.False(t, otherHandle.Has(acc.address))
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Lifecycle
// ----------------------------------------------------------------------------

func TestRecheckMempool_StartClose(t *testing.T) {
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := &recheckMockPool{}
	ctx := newRecheckTestContext()
	bc := newMockContextProvider(ctx)
	rc := newMockRechecker(ctx, noopAnteHandler)

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, newTestRecheckedTxs(), bc)

	mp.Start(testHeader(0))

	closeDone := make(chan error)
	go func() {
		closeDone <- mp.Close()
	}()

	select {
	case err := <-closeDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Close() did not return in time")
	}
}

func TestRecheckMempool_CloseIdempotent(t *testing.T) {
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := &recheckMockPool{}
	ctx := newRecheckTestContext()
	bc := newMockContextProvider(ctx)
	rc := newMockRechecker(ctx, noopAnteHandler)

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, newTestRecheckedTxs(), bc)
	mp.Start(testHeader(0))

	require.NoError(t, mp.Close())
	require.NoError(t, mp.Close())
}

func TestRecheckMempool_TriggerRecheckAfterShutdown(t *testing.T) {
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := &recheckMockPool{}
	ctx := newRecheckTestContext()
	bc := newMockContextProvider(ctx)
	rc := newMockRechecker(ctx, noopAnteHandler)

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, newTestRecheckedTxs(), bc)
	mp.Start(testHeader(0))
	require.NoError(t, mp.Close())

	done := mp.TriggerRecheck(testHeader(1))
	select {
	case <-done:
		// Expected - returns immediately after shutdown
	case <-time.After(100 * time.Millisecond):
		t.Fatal("TriggerRecheck after shutdown should return immediately")
	}
}

// ----------------------------------------------------------------------------
// Cancellation Tests
// ----------------------------------------------------------------------------

func TestRecheckMempool_ShutdownDuringRecheck(t *testing.T) {
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := &recheckMockPool{}
	ctx := newRecheckTestContext()
	bc := newMockContextProvider(ctx)

	gate := make(chan struct{})
	ready := make(chan struct{}) // signals when ante handler is waiting
	var insertCount, recheckCount atomic.Int32

	numTxsToInsert := int32(10)

	anteHandler := func(ctx sdk.Context, tx sdk.Tx, _ bool) (sdk.Context, error) {
		if insertCount.Add(1) <= numTxsToInsert {
			return ctx, nil
		}
		ready <- struct{}{} // signal we're waiting
		<-gate
		recheckCount.Add(1)
		return ctx, nil
	}

	rc := newMockRechecker(ctx, anteHandler)

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, newTestRecheckedTxs(), bc)
	mp.Start(testHeader(0))

	for range numTxsToInsert {
		key, _ := crypto.GenerateKey()
		tx := newRecheckTestTx(t, key)
		require.NoError(t, mp.Insert(ctx, tx))
	}

	recheckDone := mp.TriggerRecheck(testHeader(1))

	<-ready            // tx 1 is waiting
	gate <- struct{}{} // release tx 1
	<-ready            // tx 2 is waiting
	gate <- struct{}{} // release tx 2
	<-ready            // tx 3 is waiting - now call Close

	closeDone := make(chan error)
	go func() {
		closeDone <- mp.Close() // this will close cancelCh, then wait for recheck
	}()

	// Give Close() time to signal cancellation before unblocking
	time.Sleep(10 * time.Millisecond)

	close(gate) // unblock tx 3 and any others

	select {
	case err := <-closeDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Close() blocked during active recheck")
	}

	<-recheckDone

	finalCount := recheckCount.Load()
	require.GreaterOrEqual(t, finalCount, int32(2), "at least 2 txs should have been checked")
	require.Less(t, finalCount, numTxsToInsert, "recheck should have been cancelled before all txs")
}

// ----------------------------------------------------------------------------
// Error Path Tests
// ----------------------------------------------------------------------------

func TestRecheckMempool_GetCtxError(t *testing.T) {
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := &recheckMockPool{}
	ctx := newRecheckTestContext()

	// Blockchain that returns an error from GetLatestContext
	bc := newMockContextProvider(ctx)
	bc.ctxErr = errors.New("context unavailable")

	rc := newMockRechecker(ctx, noopAnteHandler)

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, newTestRecheckedTxs(), bc)
	mp.Start(testHeader(0))
	defer mp.Close()

	acc := newRecheckTestAccount(t)
	tx := newRecheckTestTx(t, acc.key)

	// Use a separate mempool to insert the tx (which shares the same underlying pool)
	insertBc := newMockContextProvider(ctx)
	insertRc := newMockRechecker(ctx, noopAnteHandler)
	insertMp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, insertRc, newTestRecheckedTxs(), insertBc)
	require.NoError(t, insertMp.Insert(ctx, tx))

	require.Equal(t, 1, mp.CountTx())

	mp.TriggerRecheckSync(testHeader(1))

	require.Equal(t, 1, mp.CountTx(), "tx should remain when getCtx fails")
}

func TestRecheckMempool_RemoveErrorDuringRecheck(t *testing.T) {
	acc := newRecheckTestAccount(t)
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := &recheckMockPool{}
	ctx := newRecheckTestContext()
	bc := newMockContextProvider(ctx)

	failOnRecheck := false
	anteHandler := func(ctx sdk.Context, tx sdk.Tx, _ bool) (sdk.Context, error) {
		if failOnRecheck {
			return sdk.Context{}, errors.New("recheck failed")
		}
		return ctx, nil
	}

	rc := newMockRechecker(ctx, anteHandler)

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, newTestRecheckedTxs(), bc)
	mp.Start(testHeader(0))
	defer mp.Close()

	tx := newRecheckTestTx(t, acc.key)
	require.NoError(t, mp.Insert(ctx, tx))

	failOnRecheck = true
	pool.removeErr = errors.New("remove failed")

	mp.TriggerRecheckSync(testHeader(1))

	require.Equal(t, 1, mp.CountTx(), "tx remains when remove fails")
}

// ----------------------------------------------------------------------------
// Concurrency Tests
// ----------------------------------------------------------------------------

func TestRecheckMempool_ConcurrentTriggers(t *testing.T) {
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := &recheckMockPool{}
	ctx := newRecheckTestContext()
	bc := newMockContextProvider(ctx)
	rc := newMockRechecker(ctx, noopAnteHandler)

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, newTestRecheckedTxs(), bc)
	mp.Start(testHeader(0))
	defer mp.Close()

	numTxs := 5
	for range numTxs {
		key, _ := crypto.GenerateKey()
		tx := newRecheckTestTx(t, key)
		require.NoError(t, mp.Insert(ctx, tx))
	}

	var wg sync.WaitGroup
	var timeouts atomic.Int32
	numTriggers := 10
	for range numTriggers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			done := mp.TriggerRecheck(testHeader(1))
			select {
			case <-done:
				// Expected - recheck completed
			case <-time.After(5 * time.Second):
				timeouts.Add(1)
			}
		}()
	}

	wg.Wait()
	require.Zero(t, timeouts.Load(), "some rechecks did not complete in time")
}

// ----------------------------------------------------------------------------
// Integration
// ----------------------------------------------------------------------------

func TestKrakatoaMempool_Recheck(t *testing.T) {
	type accountTx struct {
		account int
		nonce   uint64
	}

	tests := []struct {
		name           string
		insertTxs      []accountTx
		failTxs        []accountTx
		expectedRemain []accountTx
	}{
		{
			name: "single account middle nonce fails - removes it and higher nonces",
			insertTxs: []accountTx{
				{account: 0, nonce: 0},
				{account: 0, nonce: 1},
				{account: 0, nonce: 2},
			},
			failTxs: []accountTx{
				{account: 0, nonce: 1},
			},
			expectedRemain: []accountTx{
				{account: 0, nonce: 0},
			},
		},
		{
			name: "single account first nonce fails - removes all",
			insertTxs: []accountTx{
				{account: 0, nonce: 0},
				{account: 0, nonce: 1},
				{account: 0, nonce: 2},
			},
			failTxs: []accountTx{
				{account: 0, nonce: 0},
			},
			expectedRemain: []accountTx{},
		},
		{
			name: "single account last nonce fails - keeps earlier nonces",
			insertTxs: []accountTx{
				{account: 0, nonce: 0},
				{account: 0, nonce: 1},
				{account: 0, nonce: 2},
			},
			failTxs: []accountTx{
				{account: 0, nonce: 2},
			},
			expectedRemain: []accountTx{
				{account: 0, nonce: 0},
				{account: 0, nonce: 1},
			},
		},
		{
			name: "multiple accounts - failure in one does not affect others",
			insertTxs: []accountTx{
				{account: 0, nonce: 0},
				{account: 0, nonce: 1},
				{account: 0, nonce: 2},
				{account: 1, nonce: 0},
				{account: 1, nonce: 1},
				{account: 2, nonce: 0},
			},
			failTxs: []accountTx{
				{account: 0, nonce: 1},
			},
			expectedRemain: []accountTx{
				{account: 0, nonce: 0},
				{account: 1, nonce: 0},
				{account: 1, nonce: 1},
				{account: 2, nonce: 0},
			},
		},
		{
			name: "multiple accounts with multiple failures",
			insertTxs: []accountTx{
				{account: 0, nonce: 0},
				{account: 0, nonce: 1},
				{account: 1, nonce: 0},
				{account: 1, nonce: 1},
				{account: 2, nonce: 0},
			},
			failTxs: []accountTx{
				{account: 0, nonce: 0},
				{account: 1, nonce: 1},
			},
			expectedRemain: []accountTx{
				{account: 1, nonce: 0},
				{account: 2, nonce: 0},
			},
		},
		{
			name: "no failures - all txs remain",
			insertTxs: []accountTx{
				{account: 0, nonce: 0},
				{account: 0, nonce: 1},
				{account: 1, nonce: 0},
			},
			failTxs: []accountTx{},
			expectedRemain: []accountTx{
				{account: 0, nonce: 0},
				{account: 0, nonce: 1},
				{account: 1, nonce: 0},
			},
		},
		{
			name: "all txs fail",
			insertTxs: []accountTx{
				{account: 0, nonce: 0},
				{account: 1, nonce: 0},
			},
			failTxs: []accountTx{
				{account: 0, nonce: 0},
				{account: 1, nonce: 0},
			},
			expectedRemain: []accountTx{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mp, s := setupKrakatoaMempoolWithAccounts(t, 3)
			txConfig, cosmosRechecker, accounts := s.txConfig, s.cosmosRechecker, s.accounts

			getSignerAddr := func(accountIdx int) []byte {
				pubKeyBytes := crypto.CompressPubkey(&accounts[accountIdx].key.PublicKey)
				pubKey := &ethsecp256k1.PubKey{Key: pubKeyBytes}
				return pubKey.Address().Bytes()
			}

			for _, tx := range tc.insertTxs {
				cosmosTx := createTestCosmosTx(t, txConfig, accounts[tx.account].key, tx.nonce)
				require.NoError(t, mp.Insert(context.Background(), cosmosTx))
			}

			require.Equal(t, len(tc.insertTxs), mp.CountTx(), "should have all txs inserted")

			failSet := make(map[string]bool)
			for _, fail := range tc.failTxs {
				signerAddr := getSignerAddr(fail.account)
				failSet[fmt.Sprintf("%x-%d", signerAddr, fail.nonce)] = true
			}

			cosmosRechecker.SetCosmosRecheck(func(ctx sdk.Context, tx sdk.Tx) (sdk.Context, error) {
				if sigTx, ok := tx.(authsigning.SigVerifiableTx); ok {
					signers, _ := sigTx.GetSigners()
					sigs, _ := sigTx.GetSignaturesV2()
					if len(signers) > 0 && len(sigs) > 0 {
						key := fmt.Sprintf("%x-%d", signers[0], sigs[0].Sequence)
						if failSet[key] {
							return sdk.Context{}, errors.New("ante check failed")
						}
					}
				}
				return ctx, nil
			})

			mp.RecheckCosmosTxs(testHeader(1))

			require.Equal(t, len(tc.expectedRemain), mp.CountTx(),
				"expected %d txs to remain, got %d", len(tc.expectedRemain), mp.CountTx())
		})
	}
}

// ----------------------------------------------------------------------------
// Height Sync'd Store Tests
// ----------------------------------------------------------------------------

func TestRecheckMempool_RecheckedTxs(t *testing.T) {
	tests := []struct {
		name          string
		numTxs        int
		failTxIndices []int // which tx indices fail the ante handler on recheck
	}{
		{
			name:          "all txs pass",
			numTxs:        3,
			failTxIndices: []int{},
		},
		{
			name:          "one tx fails on recheck",
			numTxs:        3,
			failTxIndices: []int{1},
		},
		{
			name:          "all txs fail on recheck",
			numTxs:        3,
			failTxIndices: []int{0, 1, 2},
		},
		{
			name:          "empty pool",
			numTxs:        0,
			failTxIndices: []int{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tracker := reserver.NewReservationTracker()
			handle := tracker.NewHandle(1)
			pool := &recheckMockPool{}
			ctx := newRecheckTestContext()
			bc := newMockContextProvider(ctx)
			recheckedTxs := newTestRecheckedTxs()

			failSet := make(map[sdk.Tx]bool)
			failOnRecheck := false
			anteHandler := func(ctx sdk.Context, tx sdk.Tx, _ bool) (sdk.Context, error) {
				if failOnRecheck && failSet[tx] {
					return sdk.Context{}, errors.New("recheck failed")
				}
				return ctx, nil
			}

			rc := newMockRechecker(ctx, anteHandler)

			mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, recheckedTxs, bc)
			mp.Start(testHeader(0))
			defer mp.Close()

			txs := make([]sdk.Tx, tc.numTxs)
			for i := range tc.numTxs {
				key, _ := crypto.GenerateKey()
				txs[i] = newRecheckTestTx(t, key)
				require.NoError(t, mp.Insert(ctx, txs[i]))
			}

			for _, idx := range tc.failTxIndices {
				failSet[txs[idx]] = true
			}
			failOnRecheck = true

			header := testHeader(1)
			mp.TriggerRecheckSync(header)

			expectedCount := tc.numTxs - len(tc.failTxIndices)
			require.Equal(t, expectedCount, mp.CountTx())

			iter := mp.RecheckedTxs(context.Background(), header.Number)
			rechecked := collectIteratorTxs(iter)
			require.Len(t, rechecked, expectedCount)

			for i, tx := range txs {
				if failSet[tx] {
					require.NotContains(t, rechecked, tx, "failed tx %d should not be in rechecked store", i)
				} else {
					require.Contains(t, rechecked, tx, "passing tx %d should be in rechecked store", i)
				}
			}
		})
	}
}

func TestRecheckMempool_RecheckedTxsReset(t *testing.T) {
	tests := []struct {
		name                 string
		numInitialTxs        int
		removeBetweenHeights []int // indices of txs to remove between height 1 and height 2
	}{
		{
			name:                 "remove one tx between heights",
			numInitialTxs:        3,
			removeBetweenHeights: []int{2},
		},
		{
			name:                 "remove all txs between heights",
			numInitialTxs:        2,
			removeBetweenHeights: []int{0, 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tracker := reserver.NewReservationTracker()
			handle := tracker.NewHandle(1)
			pool := &recheckMockPool{}
			ctx := newRecheckTestContext()
			bc := newMockContextProvider(ctx)
			recheckedTxs := newTestRecheckedTxs()
			rc := newMockRechecker(ctx, noopAnteHandler)

			mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, recheckedTxs, bc)
			mp.Start(testHeader(0))
			defer mp.Close()

			txs := make([]sdk.Tx, tc.numInitialTxs)
			for i := range tc.numInitialTxs {
				key, _ := crypto.GenerateKey()
				txs[i] = newRecheckTestTx(t, key)
				require.NoError(t, mp.Insert(ctx, txs[i]))
			}

			// Recheck at height 1 - all txs pass
			mp.TriggerRecheckSync(testHeader(1))
			iter1 := mp.RecheckedTxs(context.Background(), big.NewInt(1))
			rechecked1 := collectIteratorTxs(iter1)
			require.Len(t, rechecked1, tc.numInitialTxs)

			// Remove txs between heights (simulating block inclusion)
			removed := make(map[int]bool)
			for _, idx := range tc.removeBetweenHeights {
				require.NoError(t, mp.Remove(txs[idx]))
				removed[idx] = true
			}

			// Recheck at height 2 - store should be fresh
			mp.TriggerRecheckSync(testHeader(2))
			iter2 := mp.RecheckedTxs(context.Background(), big.NewInt(2))
			rechecked2 := collectIteratorTxs(iter2)
			require.Len(t, rechecked2, tc.numInitialTxs-len(tc.removeBetweenHeights))

			for i, tx := range txs {
				if removed[i] {
					require.NotContains(t, rechecked2, tx, "removed tx %d should not be in height 2 store", i)
				} else {
					require.Contains(t, rechecked2, tx, "tx %d should be in height 2 store", i)
				}
			}
		})
	}
}

func TestRecheckMempool_RecheckedTxsBlocksUntilComplete(t *testing.T) {
	acc := newRecheckTestAccount(t)
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := &recheckMockPool{}
	ctx := newRecheckTestContext()
	bc := newMockContextProvider(ctx)
	recheckedTxs := newTestRecheckedTxs()

	var callCount atomic.Int32
	gate := make(chan struct{})
	anteHandler := func(ctx sdk.Context, tx sdk.Tx, _ bool) (sdk.Context, error) {
		if callCount.Add(1) > 1 {
			// Second call is from recheck - block until gate is released
			<-gate
		}
		return ctx, nil
	}

	rc := newMockRechecker(ctx, anteHandler)

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, recheckedTxs, bc)
	mp.Start(testHeader(0))
	defer mp.Close()

	tx := newRecheckTestTx(t, acc.key)
	require.NoError(t, mp.Insert(ctx, tx))

	header := testHeader(1)
	recheckDone := mp.TriggerRecheck(header)

	// RecheckedTxs should block because recheck is in progress
	result := make(chan sdkmempool.Iterator, 1)
	go func() {
		result <- mp.RecheckedTxs(context.Background(), header.Number)
	}()

	select {
	case <-result:
		t.Fatal("RecheckedTxs should block until recheck completes")
	case <-time.After(100 * time.Millisecond):
		// Expected - still blocking
	}

	// Release the gate to let recheck complete
	close(gate)

	select {
	case iter := <-result:
		rechecked := collectIteratorTxs(iter)
		require.Len(t, rechecked, 1, "should have 1 rechecked tx")
		require.Equal(t, tx, rechecked[0])
	case <-time.After(2 * time.Second):
		t.Fatal("RecheckedTxs did not return after recheck completed")
	}

	<-recheckDone
}

func TestRecheckMempool_RecheckerNoContextOnInsert(t *testing.T) {
	acc := newRecheckTestAccount(t)
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := &recheckMockPool{}

	// context provided has a valid context ready
	ctx := newRecheckTestContext()
	bc := newMockContextProvider(ctx)
	// rechecker does not have a valid context
	rc := newMockRechecker(sdk.Context{}, noopAnteHandler)

	recheckedTxs := newTestRecheckedTxs()
	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, recheckedTxs, bc)

	tx := newRecheckTestTx(t, acc.key)
	require.NoError(t, mp.Insert(ctx, tx))
	require.Equal(t, 1, mp.CountTx())
}

// ----------------------------------------------------------------------------
// Shared Insert/Recheck State Tests
// ----------------------------------------------------------------------------

func newRecheckTestTxWithNonce(t *testing.T, key *ecdsa.PrivateKey, nonce uint64) sdk.Tx {
	t.Helper()
	return &recheckTestTx{key: key, sequence: nonce}
}

func newRecheckTestTxWithGasPrice(t *testing.T, key *ecdsa.PrivateKey, nonce uint64, gasPrice int64) sdk.Tx {
	t.Helper()
	return &recheckTestTx{
		key:      key,
		sequence: nonce,
		gas:      100_000,
		fee:      sdk.NewCoins(sdk.NewInt64Coin(recheckTestFeeDenom, gasPrice*100_000)),
	}
}

// newNonceTrackingAnteHandler returns an ante handler that enforces sequential
// nonce ordering per account. Nonces are tracked in a map keyed by signer
// address — each successful call increments the expected nonce.
func newNonceTrackingAnteHandler() sdk.AnteHandler {
	nonces := make(map[string]uint64)
	return func(ctx sdk.Context, tx sdk.Tx, _ bool) (sdk.Context, error) {
		sigTx, ok := tx.(authsigning.SigVerifiableTx)
		if !ok {
			return ctx, nil
		}
		sigs, err := sigTx.GetSignaturesV2()
		if err != nil || len(sigs) == 0 {
			return ctx, nil
		}
		for _, sig := range sigs {
			addr := sig.PubKey.Address().String()
			expected := nonces[addr]
			if sig.Sequence != expected {
				return sdk.Context{}, fmt.Errorf(
					"account %s: expected nonce %d, got %d",
					addr, expected, sig.Sequence,
				)
			}
			nonces[addr] = expected + 1
		}
		return ctx, nil
	}
}

// TestRecheckMempool_InsertSequentialNonces verifies that inserting txs with
// sequential nonces from the same account succeeds, because each Insert
// commits the nonce increment back to the Rechecker's context.
func TestRecheckMempool_InsertSequentialNonces(t *testing.T) {
	ctx := newRecheckTestContext()
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := &recheckMockPool{}
	bc := newMockContextProvider(ctx)
	rc := newMockRechecker(ctx, newNonceTrackingAnteHandler())

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, newTestRecheckedTxs(), bc)

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	for nonce := uint64(0); nonce < 3; nonce++ {
		tx := newRecheckTestTxWithNonce(t, key, nonce)
		err := mp.Insert(ctx, tx)
		require.NoError(t, err, "insert nonce %d should succeed", nonce)
	}
	require.Equal(t, 3, mp.CountTx())
}

// TestRecheckMempool_InsertNonceGapFails verifies that skipping a nonce is
// rejected by the Rechecker's ante handler.
func TestRecheckMempool_InsertNonceGapFails(t *testing.T) {
	ctx := newRecheckTestContext()
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := &recheckMockPool{}
	bc := newMockContextProvider(ctx)
	rc := newMockRechecker(ctx, newNonceTrackingAnteHandler())

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, newTestRecheckedTxs(), bc)

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	require.NoError(t, mp.Insert(ctx, newRecheckTestTxWithNonce(t, key, 0)))

	err = mp.Insert(ctx, newRecheckTestTxWithNonce(t, key, 2))
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected nonce 1, got 2")
	require.Equal(t, 1, mp.CountTx())
}

// TestRecheckMempool_InsertAfterRecheck verifies that after a recheck commits
// surviving txs' state back to the Rechecker, a new insert at the next nonce
// succeeds.
func TestRecheckMempool_InsertAfterRecheck(t *testing.T) {
	ctx := newRecheckTestContext()
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := &recheckMockPool{}
	bc := newMockContextProvider(ctx)

	// The nonce tracker is shared across insert and recheck calls.
	// After recheck re-validates nonces 0 and 1, the tracker expects 2 next.
	rc := newMockRechecker(ctx, newNonceTrackingAnteHandler())

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, newTestRecheckedTxs(), bc)
	mp.Start(testHeader(0))
	defer mp.Close()

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	require.NoError(t, mp.Insert(ctx, newRecheckTestTxWithNonce(t, key, 0)))
	require.NoError(t, mp.Insert(ctx, newRecheckTestTxWithNonce(t, key, 1)))
	require.Equal(t, 2, mp.CountTx())

	// Reset the ante handler — recheck must re-validate from nonce 0.
	rc.anteHandler = newNonceTrackingAnteHandler()

	mp.TriggerRecheckSync(testHeader(1))
	require.Equal(t, 2, mp.CountTx())

	// Insert nonce 2 — should succeed because the recheck re-established
	// nonces 0→1, so the tracker now expects 2.
	require.NoError(t, mp.Insert(ctx, newRecheckTestTxWithNonce(t, key, 2)))
	require.Equal(t, 3, mp.CountTx())
}

func TestRecheckMempool_InsertReplacementInvalidatesRechecked(t *testing.T) {
	ctx := newRecheckTestContext()
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := &recheckMockPool{}
	bc := newMockContextProvider(ctx)
	rc := newMockRechecker(ctx, noopAnteHandler)
	recheckedTxs := newTestRecheckedTxs()

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, recheckedTxs, bc)
	mp.Start(testHeader(0))
	t.Cleanup(func() {
		require.NoError(t, mp.Close())
	})

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	tx4 := newRecheckTestTxWithNonce(t, key, 4)
	tx5 := newRecheckTestTxWithNonce(t, key, 5)
	tx6 := newRecheckTestTxWithNonce(t, key, 6)

	recheckedTxs.Do(func(store *mempool.CosmosTxStore) {
		store.AddTx(tx4)
		store.AddTx(tx5)
		store.AddTx(tx6)
	})

	replacement := newRecheckTestTxWithNonce(t, key, 4)
	require.NoError(t, mp.Insert(ctx, replacement))

	iter := mp.RecheckedTxs(context.Background(), big.NewInt(0))
	rechecked := collectIteratorTxs(iter)
	require.Empty(t, rechecked)
	require.Equal(t, 1, mp.CountTx())
}

func TestRecheckMempool_RecheckRebuildsSnapshotAfterReplacement(t *testing.T) {
	ctx := newRecheckTestContext()
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := sdkmempool.NewPriorityMempool(sdkmempool.PriorityNonceMempoolConfig[sdkmath.Int]{
		TxPriority: sdkmempool.TxPriority[sdkmath.Int]{
			GetTxPriority: func(goCtx context.Context, tx sdk.Tx) sdkmath.Int {
				_ = sdk.UnwrapSDKContext(goCtx)
				cosmosTxFee, ok := tx.(sdk.FeeTx)
				if !ok {
					return sdkmath.ZeroInt()
				}
				found, coin := cosmosTxFee.GetFee().Find(recheckTestFeeDenom)
				if !found {
					return sdkmath.ZeroInt()
				}

				gasPrice := coin.Amount.Quo(sdkmath.NewIntFromUint64(cosmosTxFee.GetGas()))
				return gasPrice
			},
			Compare: func(a, b sdkmath.Int) int {
				return a.BigInt().Cmp(b.BigInt())
			},
			MinValue: sdkmath.ZeroInt(),
		},
		TxReplacement: func(op, np sdkmath.Int, _ sdk.Tx, _ sdk.Tx) bool {
			return np.GT(op)
		},
	})
	bc := newMockContextProvider(ctx)
	rc := newMockRechecker(ctx, noopAnteHandler)
	recheckedTxs := newTestRecheckedTxs()

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, rc, recheckedTxs, bc)
	mp.Start(testHeader(0))
	t.Cleanup(func() {
		require.NoError(t, mp.Close())
	})

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	tx3 := newRecheckTestTxWithGasPrice(t, key, 3, 1)
	tx4 := newRecheckTestTxWithGasPrice(t, key, 4, 1)
	tx5 := newRecheckTestTxWithGasPrice(t, key, 5, 1)
	tx6 := newRecheckTestTxWithGasPrice(t, key, 6, 1)
	replacement := newRecheckTestTxWithGasPrice(t, key, 4, 2)

	for _, tx := range []sdk.Tx{tx3, tx4, tx5, tx6} {
		require.NoError(t, mp.Insert(ctx, tx))
	}

	// insert the replacement, which should invalidate the other txs in the pool with greater nonce.
	require.NoError(t, mp.Insert(ctx, replacement))

	iter := mp.RecheckedTxs(context.Background(), big.NewInt(0))
	rechecked := collectIteratorTxs(iter)
	require.Len(t, rechecked, 1)
	require.Equal(t, tx3, rechecked[0])

	mp.TriggerRecheckSync(testHeader(1))

	iter = mp.RecheckedTxs(context.Background(), big.NewInt(1))
	rechecked = collectIteratorTxs(iter)
	require.Equal(t, []sdk.Tx{tx3, replacement, tx5, tx6}, rechecked)
}

// newRecheckTestTx creates a minimal sdk.Tx for unit testing RecheckMempool.
func newRecheckTestTx(t *testing.T, key *ecdsa.PrivateKey) sdk.Tx {
	t.Helper()
	return &recheckTestTx{key: key}
}

// recheckTestTx is a minimal sdk.Tx implementation for unit testing.
type recheckTestTx struct {
	key      *ecdsa.PrivateKey
	sequence uint64
	gas      uint64
	fee      sdk.Coins
}

const recheckTestFeeDenom = "atest"

func (m *recheckTestTx) GetMsgs() []sdk.Msg { return nil }

func (m *recheckTestTx) GetMsgsV2() ([]proto.Message, error) {
	return nil, nil
}

func (m *recheckTestTx) GetGas() uint64 {
	if m.gas == 0 {
		return 100_000
	}
	return m.gas
}

func (m *recheckTestTx) GetFee() sdk.Coins {
	if len(m.fee) == 0 {
		return sdk.NewCoins(sdk.NewInt64Coin(recheckTestFeeDenom, 100_000))
	}
	return m.fee
}

func (m *recheckTestTx) FeePayer() []byte {
	signers, err := m.GetSigners()
	if err != nil || len(signers) == 0 {
		return nil
	}
	return signers[0]
}

func (m *recheckTestTx) FeeGranter() []byte {
	return nil
}

func (m *recheckTestTx) GetSigners() ([][]byte, error) {
	pubKeyBytes := crypto.CompressPubkey(&m.key.PublicKey)
	pubKey := &ethsecp256k1.PubKey{Key: pubKeyBytes}
	return [][]byte{pubKey.Address().Bytes()}, nil
}

func (m *recheckTestTx) GetPubKeys() ([]cryptotypes.PubKey, error) {
	pubKeyBytes := crypto.CompressPubkey(&m.key.PublicKey)
	pubKey := &ethsecp256k1.PubKey{Key: pubKeyBytes}
	return []cryptotypes.PubKey{pubKey}, nil
}

func (m *recheckTestTx) GetSignaturesV2() ([]signingtypes.SignatureV2, error) {
	pubKeyBytes := crypto.CompressPubkey(&m.key.PublicKey)
	pubKey := &ethsecp256k1.PubKey{Key: pubKeyBytes}
	return []signingtypes.SignatureV2{
		{
			PubKey:   pubKey,
			Sequence: m.sequence,
		},
	}, nil
}

// recheckMockPool is a simple in-memory ExtMempool for testing RecheckMempool in isolation.
type recheckMockPool struct {
	mu          sync.Mutex
	txs         []sdk.Tx
	insertErr   error
	removeErr   error
	insertDelay time.Duration
}

func (m *recheckMockPool) Insert(_ context.Context, tx sdk.Tx) error {
	if m.insertDelay > 0 {
		time.Sleep(m.insertDelay)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.insertErr != nil {
		return m.insertErr
	}
	m.txs = append(m.txs, tx)
	return nil
}

func (m *recheckMockPool) Remove(tx sdk.Tx) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.removeErr != nil {
		return m.removeErr
	}
	for i, t := range m.txs {
		if t == tx {
			m.txs = append(m.txs[:i], m.txs[i+1:]...)
			return nil
		}
	}
	return sdkmempool.ErrTxNotFound
}

func (m *recheckMockPool) RemoveWithReason(_ context.Context, tx sdk.Tx, _ sdkmempool.RemoveReason) error {
	return m.Remove(tx)
}

func (m *recheckMockPool) Select(_ context.Context, _ [][]byte) sdkmempool.Iterator {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.txs) == 0 {
		return nil
	}
	return &recheckMockIterator{txs: append([]sdk.Tx{}, m.txs...), idx: 0}
}

func (m *recheckMockPool) SelectBy(_ context.Context, _ [][]byte, callback func(sdk.Tx) bool) {
	m.mu.Lock()
	txsCopy := append([]sdk.Tx{}, m.txs...)
	m.mu.Unlock()

	for _, tx := range txsCopy {
		if !callback(tx) {
			return
		}
	}
}

func (m *recheckMockPool) CountTx() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.txs)
}

type recheckMockIterator struct {
	txs []sdk.Tx
	idx int
}

func (i *recheckMockIterator) Tx() sdk.Tx {
	if i.idx >= len(i.txs) {
		return nil
	}
	return i.txs[i.idx]
}

func (i *recheckMockIterator) Next() sdkmempool.Iterator {
	i.idx++
	if i.idx >= len(i.txs) {
		return nil
	}
	return i
}

// recheckTestAccount holds test account data.
type recheckTestAccount struct {
	key     *ecdsa.PrivateKey
	address common.Address
}

func newRecheckTestAccount(t *testing.T) recheckTestAccount {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	addr := crypto.PubkeyToAddress(key.PublicKey)
	return recheckTestAccount{key: key, address: addr}
}

func newRecheckTestContext() sdk.Context {
	storeKey := storetypes.NewKVStoreKey("test")
	transientKey := storetypes.NewTransientStoreKey("transient_test")
	return testutil.DefaultContext(storeKey, transientKey)
}

func noopAnteHandler(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
	return ctx, nil
}

// newTestRecheckedTxs creates a HeightSync[CosmosTxStore] for testing, starting at height 0.
func newTestRecheckedTxs() *heightsync.HeightSync[mempool.CosmosTxStore] {
	return heightsync.New(big.NewInt(0), mempool.NewCosmosTxStore, log.NewNopLogger())
}

// collectIteratorTxs drains an sdkmempool.Iterator into a slice.
func collectIteratorTxs(iter sdkmempool.Iterator) []sdk.Tx {
	var txs []sdk.Tx
	for iter != nil {
		tx := iter.Tx()
		if tx == nil {
			break
		}
		txs = append(txs, tx)
		iter = iter.Next()
	}
	return txs
}
