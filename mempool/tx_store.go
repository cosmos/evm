package mempool

import (
	"fmt"
	"strings"
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
	keys  map[string]int

	mu sync.RWMutex
}

// NewCosmosTxStore creates a new CosmosTxStore.
func NewCosmosTxStore() *CosmosTxStore {
	return &CosmosTxStore{
		index: make(map[sdk.Tx]int),
		keys:  make(map[string]int),
	}
}

// AddTx adds a single tx to the store. Duplicate txs (by pointer identity)
// are ignored. Transactions with the same signer/nonce tuple overwrite the
// existing entry to mirror the SDK PriorityNonceMempool replacement model.
func (s *CosmosTxStore) AddTx(tx sdk.Tx) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key, ok := cosmosTxKey(tx); ok {
		if idx, exists := s.keys[key]; exists {
			delete(s.index, s.txs[idx])
			s.txs[idx] = tx
			s.index[tx] = idx
			return
		}
		s.keys[key] = len(s.txs)
	}

	if _, exists := s.index[tx]; exists {
		return
	}
	s.index[tx] = len(s.txs)
	s.txs = append(s.txs, tx)
}

func cosmosTxKey(tx sdk.Tx) (string, bool) {
	signerSeqs, err := extractSignerSequences(tx)
	if err != nil || len(signerSeqs) == 0 {
		return "", false
	}

	var b strings.Builder
	for i, sig := range signerSeqs {
		if i > 0 {
			b.WriteByte('|')
		}
		nonce, err := sdkmempool.ChooseNonce(sig.seq, tx)
		if err != nil {
			return "", false
		}
		fmt.Fprintf(&b, "%s/%d", sig.account, nonce)
	}

	return b.String(), true
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
