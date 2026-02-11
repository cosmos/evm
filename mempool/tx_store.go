package mempool

import (
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// CosmosTxStore is a set of cosmos transactions that can be added to or
// removed from.
type CosmosTxStore struct {
	txs []sdk.Tx

	// index maps a tx to its position in the txs slice for fast removal
	index map[sdk.Tx]int

	mu sync.RWMutex
}

// NewCosmosTxStore creates a new CosmosTxStore.
func NewCosmosTxStore() *CosmosTxStore {
	return &CosmosTxStore{
		index: make(map[sdk.Tx]int),
	}
}

// AddTx adds a single tx to the store. Duplicate txs (by pointer identity)
// are ignored.
func (s *CosmosTxStore) AddTx(tx sdk.Tx) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.index[tx]; exists {
		return
	}
	s.index[tx] = len(s.txs)
	s.txs = append(s.txs, tx)
}

// RemoveTx removes a tx from the store using swap-with-last for O(1) removal.
func (s *CosmosTxStore) RemoveTx(tx sdk.Tx) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx, ok := s.index[tx]
	if !ok {
		return
	}

	last := len(s.txs) - 1
	if idx != last {
		s.txs[idx] = s.txs[last]
		s.index[s.txs[idx]] = idx
	}
	s.txs[last] = nil
	s.txs = s.txs[:last]
	delete(s.index, tx)
}

// Txs returns a copy of the current set of txs in the store.
func (s *CosmosTxStore) Txs() []sdk.Tx {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]sdk.Tx(nil), s.txs...)
}

// Iterator returns an sdkmempool.Iterator over the txs in the store.
func (s *CosmosTxStore) Iterator() sdkmempool.Iterator {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.txs) == 0 {
		return nil
	}

	// copy the slice so the iterator is not affected by concurrent mutations
	txs := append([]sdk.Tx(nil), s.txs...)
	return &cosmosTxIterator{txs: txs}
}

// Len returns the number of txs in the store.
func (s *CosmosTxStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.txs)
}

// cosmosTxIterator implements sdkmempool.Iterator over a slice of cosmos txs.
type cosmosTxIterator struct {
	txs []sdk.Tx
	pos int
}

func (it *cosmosTxIterator) Tx() sdk.Tx {
	return it.txs[it.pos]
}

func (it *cosmosTxIterator) Next() sdkmempool.Iterator {
	if it.pos+1 >= len(it.txs) {
		return nil
	}
	it.pos++
	return it
}
