package mempool

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// CosmosTxStore is a set of cosmos transactions that can be added to or
// removed from.
type CosmosTxStore struct {
	txs []sdk.Tx

	// index tracks pointer-identity dedupe for txs that cannot be keyed.
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

// AddTx adds a single tx to the store while constructing a validated snapshot.
// Duplicate txs (by pointer identity) are ignored. A signer/nonce collision is
// a programming error: validated snapshots must never be edited in place.
func (s *CosmosTxStore) AddTx(tx sdk.Tx) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key, ok := cosmosTxKey(tx); ok {
		if _, exists := s.keys[key]; exists {
			panic(fmt.Sprintf("duplicate cosmos tx snapshot entry for signer/nonce key %q", key))
		}
		s.keys[key] = len(s.txs)
	}

	if _, exists := s.index[tx]; exists {
		return
	}
	s.index[tx] = len(s.txs)
	s.txs = append(s.txs, tx)
}

// InvalidateFrom removes any stored tx that depends on the supplied tx's signer/nonces.
// It is used for live mempool replacements: once a tx at nonce N changes, any stored tx
// for the same signer(s) with nonce >= N is no longer known-valid for proposal building.
func (s *CosmosTxStore) InvalidateFrom(tx sdk.Tx) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	thresholds, ok := cosmosTxNonceMap(tx)
	if !ok {
		return 0
	}

	removed := 0
	nextTxs := make([]sdk.Tx, 0, len(s.txs))
	for _, existing := range s.txs {
		if invalidatesCosmosTx(existing, thresholds) {
			removed++
			continue
		}
		nextTxs = append(nextTxs, existing)
	}

	if removed == 0 {
		return 0
	}

	s.reindex(nextTxs)
	return removed
}

func cosmosTxKey(tx sdk.Tx) (string, bool) {
	nonceMap, ok := cosmosTxNonceMap(tx)
	if !ok {
		return "", false
	}

	var b strings.Builder
	first := true
	for _, sig := range sortedSignerNonces(nonceMap) {
		if !first {
			b.WriteByte('|')
		}
		first = false
		fmt.Fprintf(&b, "%s/%d", sig.account, sig.seq)
	}

	return b.String(), true
}

// cosmosTxNonceMap extracts the signers from the transaction
// and returns a signer -> nonce map.
func cosmosTxNonceMap(tx sdk.Tx) (map[string]uint64, bool) {
	signerSeqs, err := extractSignerSequences(tx)
	if err != nil || len(signerSeqs) == 0 {
		return nil, false
	}

	nonceMap := make(map[string]uint64, len(signerSeqs))
	for _, sig := range signerSeqs {
		nonce, err := sdkmempool.ChooseNonce(sig.seq, tx)
		if err != nil {
			return nil, false
		}
		nonceMap[sig.account] = nonce
	}

	return nonceMap, true
}

func sortedSignerNonces(nonceMap map[string]uint64) []signerSequence {
	signerSeqs := make([]signerSequence, 0, len(nonceMap))
	for account, seq := range nonceMap {
		signerSeqs = append(signerSeqs, signerSequence{account: account, seq: seq})
	}
	slices.SortFunc(signerSeqs, func(a, b signerSequence) int {
		return strings.Compare(a.account, b.account)
	})
	return signerSeqs
}

func invalidatesCosmosTx(tx sdk.Tx, thresholds map[string]uint64) bool {
	nonceMap, ok := cosmosTxNonceMap(tx)
	if !ok {
		return false
	}

	for account, threshold := range thresholds {
		nonce, exists := nonceMap[account]
		if exists && nonce >= threshold {
			return true
		}
	}
	return false
}

func (s *CosmosTxStore) reindex(txs []sdk.Tx) {
	s.txs = txs
	s.index = make(map[sdk.Tx]int, len(txs))
	s.keys = make(map[string]int, len(txs))
	for i, tx := range txs {
		s.index[tx] = i
		if key, ok := cosmosTxKey(tx); ok {
			s.keys[key] = i
		}
	}
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
