package mempool

import (
	"sync"
	"testing"
	"time"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
)

// mockTxPool is a mock implementation of TxPool for testing
type mockTxPool struct {
	mu     sync.Mutex
	addFn  func([]*ethtypes.Transaction, bool) []error
	txs    []*ethtypes.Transaction
	addErr error
}

func newMockTxPool() *mockTxPool {
	return &mockTxPool{
		txs: make([]*ethtypes.Transaction, 0),
	}
}

func (m *mockTxPool) Add(txs []*ethtypes.Transaction, sync bool) []error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.addFn != nil {
		return m.addFn(txs, sync)
	}

	errs := make([]error, len(txs))
	for i, tx := range txs {
		m.txs = append(m.txs, tx)
		errs[i] = m.addErr
	}
	return errs
}

func (m *mockTxPool) getTxs() []*ethtypes.Transaction {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.txs
}

func (m *mockTxPool) setAddFn(fn func([]*ethtypes.Transaction, bool) []error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addFn = fn
}

func TestInsertQueue_PushAndProcess(t *testing.T) {
	pool := newMockTxPool()
	logger := log.NewNopLogger()
	iq := newInsertQueue(pool, 1000, logger)
	defer iq.Close()

	// Create a test transaction
	tx := ethtypes.NewTransaction(1, [20]byte{0x01}, nil, 21000, nil, nil)

	// Push transaction
	iq.Push(tx, nil)

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
	pool := newMockTxPool()
	logger := log.NewNopLogger()
	iq := newInsertQueue(pool, 1000, logger)
	defer iq.Close()

	// Create multiple test transactions
	tx1 := ethtypes.NewTransaction(1, [20]byte{0x01}, nil, 21000, nil, nil)
	tx2 := ethtypes.NewTransaction(2, [20]byte{0x02}, nil, 21000, nil, nil)
	tx3 := ethtypes.NewTransaction(3, [20]byte{0x03}, nil, 21000, nil, nil)

	// Push transactions
	iq.Push(tx1, nil)
	iq.Push(tx2, nil)
	iq.Push(tx3, nil)

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
	pool := newMockTxPool()
	logger := log.NewNopLogger()
	iq := newInsertQueue(pool, 1000, logger)
	defer iq.Close()

	// Push nil transaction
	iq.Push(nil, nil)

	// Wait a bit to ensure nothing is processed
	time.Sleep(100 * time.Millisecond)

	// Verify no transaction was added
	txs := pool.getTxs()
	require.Len(t, txs, 0)
}

func TestInsertQueue_SlowAddition(t *testing.T) {
	pool := newMockTxPool()

	// Make Add slow to allow queue to back up
	pool.setAddFn(func(txs []*ethtypes.Transaction, sync bool) []error {
		time.Sleep(10 * time.Second)
		return []error{nil}
	})

	logger := log.NewNopLogger()
	iq := newInsertQueue(pool, 1000, logger)
	defer iq.Close()

	// Push first transaction to start processing
	tx1 := ethtypes.NewTransaction(1, [20]byte{0x01}, nil, 21000, nil, nil)
	iq.Push(tx1, nil)

	time.Sleep(100 * time.Millisecond)

	// Push a bunch of transactions and verify that we did not have to wait for
	// the 200 ms to add the first tx.
	start := time.Now()
	var nonce uint64
	for nonce = 0; nonce < 100; nonce++ {
		tx := ethtypes.NewTransaction(nonce+2, [20]byte{byte(nonce + 2)}, nil, 21000, nil, nil)
		iq.Push(tx, nil)
	}
	require.Less(t, time.Since(start), 100*time.Millisecond, "pushes should not block")
}

func TestInsertQueue_RejectsWhenFull(t *testing.T) {
	pool := newMockTxPool()

	// when addFn is called, push a value onto a channel to signal that a
	// single tx has been popped from the queue, then block forever so no more
	// txs can be popped, that means we can add 1 more tx then the queue will
	// be at max capacity, and adding 1 after that will trigger an error
	added := make(chan struct{}, 1)
	pool.setAddFn(func(txs []*ethtypes.Transaction, sync bool) []error {
		added <- struct{}{}
		select {} // block forever
	})

	logger := log.NewNopLogger()
	maxSize := uint64(5)
	iq := newInsertQueue(pool, maxSize, logger)
	defer iq.Close()

	// Fill the queue to capacity
	// Note: The first tx will be immediately popped and start processing (where it blocks),
	// so we need to push maxSize + 1 transactions to actually fill the queue
	for i := uint64(0); i <= maxSize; i++ {
		tx := ethtypes.NewTransaction(i+1, [20]byte{byte(i + 1)}, nil, 21000, nil, nil)
		iq.Push(tx, nil)
	}

	// wait for first tx to be popped and addFn to be called and blocking
	<-added

	// Try to push one more transaction with error channel, queue is now at max capacity
	tx := ethtypes.NewTransaction(100, [20]byte{0x64}, nil, 21000, nil, nil)
	iq.Push(tx, nil)

	// Push another tx into the full queue, should be rejected
	sub := make(chan error, 1)
	fullTx := ethtypes.NewTransaction(101, [20]byte{0x64}, nil, 21000, nil, nil)
	iq.Push(fullTx, sub)

	// Verify we got the queue full error
	select {
	case err := <-sub:
		require.ErrorIs(t, err, ErrInsertQueueFull, "should receive queue full error")
	default:
		t.Fatal("did not receive error from full queue")
	}
}
