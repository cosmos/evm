package legacypool

import (
	"math/big"
	"sort"
	"sync"

	"github.com/cosmos/evm/mempool/txpool"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/holiman/uint256"
)

var (
	// txsCollected is the total amount of txs returned by Collect.
	txsCollected = metrics.NewRegisteredMeter("legacypool/txstore/txscollected", nil)
)

// TxStore is a set of transactions at a height that can be added to or
// removed from.
type TxStore struct {
	txs map[common.Address]types.Transactions

	// lookup provides a fast lookup to determine if a tx is in the set or not
	lookup map[common.Hash]struct{}

	mu sync.RWMutex
}

// NewTxStore creates a new TxStore.
func NewTxStore() *TxStore {
	return &TxStore{
		txs:    make(map[common.Address]types.Transactions),
		lookup: make(map[common.Hash]struct{}),
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

	// Convert the new uint256.Int types to the old big.Int ones used by the legacy pool
	var (
		minTipBig  *big.Int
		baseFeeBig *big.Int
	)
	if filter.MinTip != nil {
		minTipBig = filter.MinTip.ToBig()
	}
	if filter.BaseFee != nil {
		baseFeeBig = filter.BaseFee.ToBig()
	}

	numSelected := 0
	pending := make(map[common.Address][]*txpool.LazyTransaction, len(t.txs))

	for addr, txs := range t.txs {
		sort.Sort(types.TxByNonce(txs))

		// Filter by minimum tip if configured
		if minTipBig != nil {
			for i, tx := range txs {
				if tx.EffectiveGasTipIntCmp(minTipBig, baseFeeBig) < 0 {
					txs = txs[:i]
					break
				}
			}
		}

		// Convert to lazy transactions
		if len(txs) > 0 {
			lazies := make([]*txpool.LazyTransaction, len(txs))
			for i, tx := range txs {
				lazies[i] = &txpool.LazyTransaction{
					Hash:      tx.Hash(),
					Tx:        tx,
					Time:      tx.Time(),
					GasFeeCap: uint256.MustFromBig(tx.GasFeeCap()),
					GasTipCap: uint256.MustFromBig(tx.GasTipCap()),
					Gas:       tx.Gas(),
					BlobGas:   tx.BlobGas(),
				}
			}
			numSelected += len(lazies)
			pending[addr] = lazies
		}
	}

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
	}

	if existing, ok := t.txs[addr]; ok {
		t.txs[addr] = append(existing, toAdd...)
	} else {
		t.txs[addr] = toAdd
	}
}

// AddTx adds a single tx to the store.
func (t *TxStore) AddTx(addr common.Address, tx *types.Transaction) {
	t.AddTxs(addr, types.Transactions{tx})
}

// RemoveTx removes a tx for an address from the current set.
func (t *TxStore) RemoveTx(addr common.Address, tx *types.Transaction) {
	t.mu.Lock()
	defer t.mu.Unlock()

	defer delete(t.lookup, tx.Hash())

	txs, ok := t.txs[addr]
	if !ok {
		return
	}

	// Find and remove the tx by nonce
	nonce := tx.Nonce()
	for i := 0; i < len(txs); i++ {
		if txs[i].Nonce() == nonce {
			// Swap with last element and truncate
			txs[i] = txs[len(txs)-1]
			t.txs[addr] = txs[:len(txs)-1]
			return
		}
	}
}
