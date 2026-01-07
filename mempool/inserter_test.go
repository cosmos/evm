package mempool

import (
	"sync"
	"testing"
	"time"

	"cosmossdk.io/log"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
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

func (m *mockTxPool) setAddError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addErr = err
}

func (m *mockTxPool) setAddFn(fn func([]*ethtypes.Transaction, bool) []error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addFn = fn
}

func TestInsertQueue_PushAndProcess(t *testing.T) {
	pool := newMockTxPool()
	logger := log.NewNopLogger()
	iq := newInsertQueue(pool, logger)
	defer iq.Close()

	// Create a test transaction
	tx := ethtypes.NewTransaction(1, [20]byte{0x01}, nil, 21000, nil, nil)

	// Push transaction
	iq.Push(tx)

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
	iq := newInsertQueue(pool, logger)
	defer iq.Close()

	// Create multiple test transactions
	tx1 := ethtypes.NewTransaction(1, [20]byte{0x01}, nil, 21000, nil, nil)
	tx2 := ethtypes.NewTransaction(2, [20]byte{0x02}, nil, 21000, nil, nil)
	tx3 := ethtypes.NewTransaction(3, [20]byte{0x03}, nil, 21000, nil, nil)

	// Push transactions
	iq.Push(tx1)
	iq.Push(tx2)
	iq.Push(tx3)

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
	iq := newInsertQueue(pool, logger)
	defer iq.Close()

	// Push nil transaction
	iq.Push(nil)

	// Wait a bit to ensure nothing is processed
	time.Sleep(100 * time.Millisecond)

	// Verify no transaction was added
	txs := pool.getTxs()
	require.Len(t, txs, 0)
}

func TestInsertQueue_SlowAddition(t *testing.T) {
	pool := newMockTxPool()

	// Make Add slow to allow queue to back up
	var addCalled sync.WaitGroup
	addCalled.Add(1)
	pool.setAddFn(func(txs []*ethtypes.Transaction, sync bool) []error {
		addCalled.Done()
		time.Sleep(200 * time.Millisecond)
		return []error{nil}
	})

	logger := log.NewNopLogger()
	iq := newInsertQueue(pool, logger)
	defer iq.Close()

	// Push first transaction to start processing
	tx1 := ethtypes.NewTransaction(1, [20]byte{0x01}, nil, 21000, nil, nil)
	iq.Push(tx1)

	// Wait for Add to be called
	addCalled.Wait()

	// Push a bunch of transactions and verify that we did not have to wait for
	// the 200 ms to add the first tx.
	start := time.Now()
	for i := 0; i < 100; i++ {
		tx := ethtypes.NewTransaction(uint64(i+2), [20]byte{byte(i + 2)}, nil, 21000, nil, nil)
		iq.Push(tx)
	}
	require.Less(t, time.Since(start), 100*time.Millisecond, "pushes should not block")
}
