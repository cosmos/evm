package legacypool

import (
	"sort"
	"sync"

	"cosmossdk.io/log/v2"
	"github.com/cosmos/evm/mempool/txpool"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/metrics"
)

// txsCollected is the total amount of txs returned by Collect.
var txsCollected = metrics.NewRegisteredMeter("legacypool/txstore/txscollected", nil)

// TxStore is a set of transactions at a height that can be added to or
// removed from.
type TxStore struct {
	txs map[common.Address]types.Transactions

	// lookup provides a fast lookup to determine if a tx is in the set or not
	lookup map[common.Hash]struct{}

	total uint64

	logger log.Logger
	mu     sync.RWMutex
}

// NewTxStore creates a new TxStore.
func NewTxStore(logger log.Logger) *TxStore {
	return &TxStore{
		txs:    make(map[common.Address]types.Transactions),
		total:  0,
		lookup: make(map[common.Hash]struct{}),
		logger: logger.With("txstore", "evm"),
	}
}

// Get returns the current set of txs in the store.
func (t *TxStore) Txs(filter txpool.PendingFilter) map[common.Address][]*txpool.LazyTransaction {
	// Do not support blob txs
	if filter.OnlyBlobTxs {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	numSelected := 0
	pending := make(map[common.Address][]*txpool.LazyTransaction, len(t.txs))

	for addr, txs := range t.txs {
		sort.Sort(types.TxByNonce(txs))

		if lazies := filterAndWrapTxs(txs, filter.MinTip, filter.BaseFee); len(lazies) > 0 {
			numSelected += len(lazies)
			pending[addr] = lazies
		}
	}

	t.logger.Info("collected txs from evm tx store", "total_in_store", t.total, "num_selected", numSelected)
	txsCollected.Mark(int64(numSelected))
	return pending
}

// AddTxs adds txs to the store.
func (t *TxStore) AddTxs(addr common.Address, txs types.Transactions) {
	t.mu.Lock()
	defer t.mu.Unlock()

	toAdd := make([]*types.Transaction, 0, len(txs))
	for _, tx := range txs {
		if _, exists := t.lookup[tx.Hash()]; exists {
			continue
		}
		toAdd = append(toAdd, tx)
		t.lookup[tx.Hash()] = struct{}{}
	}

	if existing, ok := t.txs[addr]; ok {
		t.txs[addr] = append(existing, toAdd...)
	} else {
		t.txs[addr] = toAdd
	}

	// mark the txs in the lookup
	for _, tx := range toAdd {
		t.lookup[tx.Hash()] = struct{}{}
	}

	t.total += uint64(len(toAdd))
}

// AddTx adds a single tx to the store.
func (t *TxStore) AddTx(addr common.Address, tx *types.Transaction) {
	t.AddTxs(addr, types.Transactions{tx})
}

// RemoveTx removes a tx for an address from the current set.
func (t *TxStore) RemoveTx(addr common.Address, tx *types.Transaction) {
	t.RemoveTxsFromNonce(addr, tx.Nonce())
}

// RemoveTxsFromNonce removes all txs for addr whose nonce is >= minNonce.
func (t *TxStore) RemoveTxsFromNonce(addr common.Address, minNonce uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	txs, ok := t.txs[addr]
	if !ok {
		return
	}

	next := txs[:0]
	numRemoved := 0
	for _, existing := range txs {
		if existing.Nonce() >= minNonce {
			delete(t.lookup, existing.Hash())
			numRemoved++
			continue
		}
		next = append(next, existing)
	}

	// memory reclaim
	clear(txs[len(next):])

	t.total -= uint64(numRemoved)
	if len(next) == 0 {
		delete(t.txs, addr)
		return
	}
	t.txs[addr] = next
}
