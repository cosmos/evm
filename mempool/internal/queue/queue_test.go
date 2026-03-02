package queue

import (
	"sync"
	"testing"
	"time"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log/v2"
)

// mockPool is a mock that records inserted transactions and optionally
// delegates to a custom function.
type mockPool struct {
	mu       sync.Mutex
	insertFn func([]*ethtypes.Transaction) []error
	txs      []*ethtypes.Transaction
}

func newMockPool() *mockPool {
	return &mockPool{
		txs: make([]*ethtypes.Transaction, 0),
	}
}

func (m *mockPool) insert(txs []*ethtypes.Transaction) []error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.insertFn != nil {
		return m.insertFn(txs)
	}

	errs := make([]error, len(txs))
	m.txs = append(m.txs, txs...)
	return errs
}

func (m *mockPool) getTxs() []*ethtypes.Transaction {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.txs
}

func (m *mockPool) setInsertFn(fn func([]*ethtypes.Transaction) []error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.insertFn = fn
}

func TestInsertQueue_PushAndProcess(t *testing.T) {
	pool := newMockPool()
	logger := log.NewNopLogger()
	iq := New[ethtypes.Transaction](pool.insert, 1000, logger)
	defer iq.Close()

	// Create a test transaction
	tx := ethtypes.NewTransaction(1, [20]byte{0x01}, nil, 21000, nil, nil)

	// Push transaction
	_ = iq.Push(tx)

	// Wait for transaction to be processed
	require.Eventually(t, func() bool {
		return len(pool.getTxs()) == 1
	}, time.Second, 10*time.Millisecond, "transaction should be processed")

	// Verify the transaction was added
	txs := pool.getTxs()
	require.Len(t, txs, 1)
	require.Equal(t, tx.Hash(), txs[0].Hash())
}

func TestInsertQueue_ProcessesMultipleTransactions(t *testing.T) {
	pool := newMockPool()
	logger := log.NewNopLogger()
	iq := New[ethtypes.Transaction](pool.insert, 1000, logger)
	defer iq.Close()

	// Create multiple test transactions
	tx1 := ethtypes.NewTransaction(1, [20]byte{0x01}, nil, 21000, nil, nil)
	tx2 := ethtypes.NewTransaction(2, [20]byte{0x02}, nil, 21000, nil, nil)
	tx3 := ethtypes.NewTransaction(3, [20]byte{0x03}, nil, 21000, nil, nil)

	// Push transactions
	_ = iq.Push(tx1)
	_ = iq.Push(tx2)
	_ = iq.Push(tx3)

	// Wait for all transactions to be processed
	require.Eventually(t, func() bool {
		return len(pool.getTxs()) == 3
	}, time.Second, 10*time.Millisecond, "all transactions should be processed")

	// Verify transactions were added in FIFO order
	txs := pool.getTxs()
	require.Len(t, txs, 3)
	require.Equal(t, tx1.Hash(), txs[0].Hash())
	require.Equal(t, tx2.Hash(), txs[1].Hash())
	require.Equal(t, tx3.Hash(), txs[2].Hash())
}

func TestInsertQueue_IgnoresNilTransaction(t *testing.T) {
	pool := newMockPool()
	logger := log.NewNopLogger()
	iq := New[ethtypes.Transaction](pool.insert, 1000, logger)
	defer iq.Close()

	// Push nil transaction
	_ = iq.Push(nil)

	// Wait a bit to ensure nothing is processed
	time.Sleep(100 * time.Millisecond)

	// Verify no transaction was added
	txs := pool.getTxs()
	require.Len(t, txs, 0)
}

func TestInsertQueue_SlowAddition(t *testing.T) {
	pool := newMockPool()

	// Make insert slow to allow queue to back up
	pool.setInsertFn(func(txs []*ethtypes.Transaction) []error {
		time.Sleep(10 * time.Second)
		return make([]error, len(txs))
	})

	logger := log.NewNopLogger()
	iq := New[ethtypes.Transaction](pool.insert, 1000, logger)
	defer iq.Close()

	// Push first transaction to start processing
	tx1 := ethtypes.NewTransaction(1, [20]byte{0x01}, nil, 21000, nil, nil)
	_ = iq.Push(tx1)

	time.Sleep(100 * time.Millisecond)

	// Push a bunch of transactions and verify that we did not have to wait for
	// the 200 ms to add the first tx.
	start := time.Now()
	var nonce uint64
	for nonce = 0; nonce < 100; nonce++ {
		tx := ethtypes.NewTransaction(nonce+2, [20]byte{byte(nonce + 2)}, nil, 21000, nil, nil)
		_ = iq.Push(tx)
	}
	require.Less(t, time.Since(start), 100*time.Millisecond, "pushes should not block")
}

func TestInsertQueue_RejectsWhenFull(t *testing.T) {
	pool := newMockPool()

	// when insertFn is called, push a value onto a channel to signal that a
	// single tx has been popped from the queue, then block forever so no more
	// txs can be popped, that means we can add 1 more tx then the queue will
	// be at max capacity, and adding 1 after that will trigger an error
	added := make(chan struct{}, 1)
	pool.setInsertFn(func(txs []*ethtypes.Transaction) []error {
		added <- struct{}{}
		select {} // block forever
	})

	logger := log.NewNopLogger()
	iq := New[ethtypes.Transaction](pool.insert, 5, logger)
	defer iq.Close()

	// This first tx will be immediately popped and start processing (where it
	// blocks)
	nonce := uint64(0)
	tx := ethtypes.NewTransaction(nonce, [20]byte{byte(nonce + 1)}, nil, 21000, nil, nil)
	_ = iq.Push(tx)
	nonce++

	// wait for first tx to be popped and insertFn to be called and blocking
	<-added

	// Fill the queue to capacity
	for ; nonce <= 5; nonce++ {
		tx := ethtypes.NewTransaction(nonce, [20]byte{byte(nonce + 1)}, nil, 21000, nil, nil)
		_ = iq.Push(tx)
	}

	// Try to push one more transaction with error channel, queue is now at max capacity
	tx = ethtypes.NewTransaction(100, [20]byte{0x64}, nil, 21000, nil, nil)
	_ = iq.Push(tx)

	// Push another tx into the full queue, should be rejected
	fullTx := ethtypes.NewTransaction(101, [20]byte{0x64}, nil, 21000, nil, nil)
	sub := iq.Push(fullTx)

	// Verify we got the queue full error
	select {
	case err := <-sub:
		require.ErrorIs(t, err, ErrQueueFull, "should receive queue full error")
	default:
		t.Fatal("did not receive error from full queue")
	}
}
