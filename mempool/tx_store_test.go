package mempool

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	"github.com/cosmos/gogoproto/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// mockTx is a minimal sdk.Tx implementation for testing.
type mockTx struct {
	id int
}

var _ sdk.Tx = (*mockTx)(nil)

func (m *mockTx) GetMsgs() []proto.Message              { return nil }
func (m *mockTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

func newMockTx(id int) sdk.Tx {
	return &mockTx{id: id}
}

func TestCosmosTxStoreAddAndGet(t *testing.T) {
	store := NewCosmosTxStore()

	tx1 := newMockTx(1)
	tx2 := newMockTx(2)
	tx3 := newMockTx(3)

	store.AddTx(tx1)
	store.AddTx(tx2)
	store.AddTx(tx3)

	txs := store.Txs()
	require.Len(t, txs, 3)
}

func TestCosmosTxStoreDedup(t *testing.T) {
	store := NewCosmosTxStore()

	tx := newMockTx(1)

	store.AddTx(tx)
	store.AddTx(tx)
	store.AddTx(tx)

	require.Equal(t, 1, store.Len())
}

func TestCosmosTxStoreRemoveTx(t *testing.T) {
	store := NewCosmosTxStore()

	tx1 := newMockTx(1)
	tx2 := newMockTx(2)
	tx3 := newMockTx(3)

	store.AddTx(tx1)
	store.AddTx(tx2)
	store.AddTx(tx3)

	store.RemoveTx(tx2)
	require.Equal(t, 2, store.Len())

	txs := store.Txs()
	for _, tx := range txs {
		require.NotEqual(t, tx, tx2)
	}
}

func TestCosmosTxStoreRemoveNonexistent(t *testing.T) {
	store := NewCosmosTxStore()

	tx1 := newMockTx(1)
	tx2 := newMockTx(2)

	store.AddTx(tx1)
	store.RemoveTx(tx2) // should be a no-op

	require.Equal(t, 1, store.Len())
}

func TestCosmosTxStoreRemoveAll(t *testing.T) {
	store := NewCosmosTxStore()

	tx1 := newMockTx(1)
	tx2 := newMockTx(2)

	store.AddTx(tx1)
	store.AddTx(tx2)
	store.RemoveTx(tx1)
	store.RemoveTx(tx2)

	require.Equal(t, 0, store.Len())
	require.Empty(t, store.Txs())
}

func TestCosmosTxStoreConcurrentRemove(t *testing.T) {
	store := NewCosmosTxStore()

	numTxs := 1000
	txs := make([]sdk.Tx, numTxs)
	for i := range txs {
		txs[i] = newMockTx(i)
		store.AddTx(txs[i])
	}

	// concurrently remove even-index txs
	var wg sync.WaitGroup
	for i := 0; i < numTxs; i += 2 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			store.RemoveTx(txs[idx])
		}(i)
	}
	wg.Wait()

	require.Equal(t, numTxs/2, store.Len())
}

func TestCosmosTxStoreIterator(t *testing.T) {
	store := NewCosmosTxStore()

	tx1 := newMockTx(1)
	tx2 := newMockTx(2)
	tx3 := newMockTx(3)

	store.AddTx(tx1)
	store.AddTx(tx2)
	store.AddTx(tx3)

	iter := store.Iterator()
	require.NotNil(t, iter)

	var collected []sdk.Tx
	for ; iter != nil; iter = iter.Next() {
		collected = append(collected, iter.Tx())
	}
	require.Len(t, collected, 3)
}

func TestCosmosTxStoreIteratorEmpty(t *testing.T) {
	store := NewCosmosTxStore()
	require.Nil(t, store.Iterator())
}

func TestCosmosTxStoreIteratorSnapshotIsolation(t *testing.T) {
	store := NewCosmosTxStore()

	tx1 := newMockTx(1)
	tx2 := newMockTx(2)

	store.AddTx(tx1)
	store.AddTx(tx2)

	iter := store.Iterator()
	require.NotNil(t, iter)

	// mutate the store after creating the iterator
	store.AddTx(newMockTx(3))
	store.RemoveTx(tx1)

	// iterator should still see the original 2 txs
	var count int
	for ; iter != nil; iter = iter.Next() {
		count++
	}
	require.Equal(t, 2, count)
}
