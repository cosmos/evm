package mempool_test

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/mempool/reserver"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
)

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
			getCtx := func() (sdk.Context, error) { return ctx, nil }

			mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, anteHandler, getCtx)

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
			getCtx := func() (sdk.Context, error) { return ctx, nil }

			mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, noopAnteHandler, getCtx)

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
	getCtx := func() (sdk.Context, error) { return ctx, nil }

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, noopAnteHandler, getCtx)

	mp.Start()

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
	getCtx := func() (sdk.Context, error) { return ctx, nil }

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, noopAnteHandler, getCtx)
	mp.Start()

	require.NoError(t, mp.Close())
	require.NoError(t, mp.Close())
}

func TestRecheckMempool_TriggerRecheckAfterShutdown(t *testing.T) {
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := &recheckMockPool{}
	ctx := newRecheckTestContext()
	getCtx := func() (sdk.Context, error) { return ctx, nil }

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, noopAnteHandler, getCtx)
	mp.Start()
	require.NoError(t, mp.Close())

	done := mp.TriggerRecheck()
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
	getCtx := func() (sdk.Context, error) { return ctx, nil }

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

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, anteHandler, getCtx)
	mp.Start()

	for range numTxsToInsert {
		key, _ := crypto.GenerateKey()
		tx := newRecheckTestTx(t, key)
		require.NoError(t, mp.Insert(ctx, tx))
	}

	recheckDone := mp.TriggerRecheck()

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

	ctxErr := errors.New("context unavailable")
	getCtx := func() (sdk.Context, error) { return sdk.Context{}, ctxErr }

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, noopAnteHandler, getCtx)
	mp.Start()
	defer mp.Close()

	acc := newRecheckTestAccount(t)
	tx := newRecheckTestTx(t, acc.key)

	insertCtx := ctx
	insertGetCtx := func() (sdk.Context, error) { return insertCtx, nil }
	insertMp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, noopAnteHandler, insertGetCtx)
	require.NoError(t, insertMp.Insert(insertCtx, tx))

	require.Equal(t, 1, mp.CountTx())

	mp.TriggerRecheckSync()

	require.Equal(t, 1, mp.CountTx(), "tx should remain when getCtx fails")
}

func TestRecheckMempool_RemoveErrorDuringRecheck(t *testing.T) {
	acc := newRecheckTestAccount(t)
	tracker := reserver.NewReservationTracker()
	handle := tracker.NewHandle(1)
	pool := &recheckMockPool{}
	ctx := newRecheckTestContext()
	getCtx := func() (sdk.Context, error) { return ctx, nil }

	failOnRecheck := false
	anteHandler := func(ctx sdk.Context, tx sdk.Tx, _ bool) (sdk.Context, error) {
		if failOnRecheck {
			return sdk.Context{}, errors.New("recheck failed")
		}
		return ctx, nil
	}

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, anteHandler, getCtx)
	mp.Start()
	defer mp.Close()

	tx := newRecheckTestTx(t, acc.key)
	require.NoError(t, mp.Insert(ctx, tx))

	failOnRecheck = true
	pool.removeErr = errors.New("remove failed")

	mp.TriggerRecheckSync()

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
	getCtx := func() (sdk.Context, error) { return ctx, nil }

	mp := mempool.NewRecheckMempool(log.NewNopLogger(), pool, handle, noopAnteHandler, getCtx)
	mp.Start()
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
			done := mp.TriggerRecheck()
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

func TestMempool_Recheck(t *testing.T) {
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
			storeKey := storetypes.NewKVStoreKey("test")
			transientKey := storetypes.NewTransientStoreKey("transient_test")
			ctx := testutil.DefaultContext(storeKey, transientKey) //nolint:staticcheck // false positive.

			failSet := make(map[string]bool)

			anteHandler := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
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
			}

			mp, _, txConfig, _, _, accounts := setupMempoolWithAnteHandler(t, anteHandler, 3)

			getSignerAddr := func(accountIdx int) []byte {
				pubKeyBytes := crypto.CompressPubkey(&accounts[accountIdx].key.PublicKey)
				pubKey := &ethsecp256k1.PubKey{Key: pubKeyBytes}
				return pubKey.Address().Bytes()
			}

			for _, tx := range tc.insertTxs {
				cosmosTx := createTestCosmosTx(t, txConfig, accounts[tx.account].key, tx.nonce)
				require.NoError(t, mp.Insert(ctx, cosmosTx))
			}

			require.Equal(t, len(tc.insertTxs), mp.CountTx(), "should have all txs inserted")

			for _, fail := range tc.failTxs {
				signerAddr := getSignerAddr(fail.account)
				failSet[fmt.Sprintf("%x-%d", signerAddr, fail.nonce)] = true
			}

			mp.RecheckCosmosTxs()

			require.Equal(t, len(tc.expectedRemain), mp.CountTx(),
				"expected %d txs to remain, got %d", len(tc.expectedRemain), mp.CountTx())
		})
	}
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
}

func (m *recheckTestTx) GetMsgs() []sdk.Msg { return nil }

func (m *recheckTestTx) GetMsgsV2() ([]proto.Message, error) {
	return nil, nil
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
