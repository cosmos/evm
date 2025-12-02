package mempool

import (
	"slices"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type reapList struct {
	txs      types.Transactions
	txLookup map[common.Hash]int
	txsLock  sync.RWMutex
	encodeTx func(tx *ethtypes.Transaction) ([]byte, error)
}

func NewReapList(encodeTx func(tx *ethtypes.Transaction) ([]byte, error)) *reapList {
	return &reapList{
		encodeTx: encodeTx,
		txLookup: make(map[common.Hash]int),
	}
}

// Reap returns a list of the oldest to newest transactions bytes from the reap
// list until either maxBytes or maxGas is reached for the list of transactions
// being returned. If maxBytes and maxGas are both 0 all txs will be returned.
//
// If encoding fails for a tx, it is removed from the reap list and is not
// returned.
func (rl *reapList) Reap(maxBytes uint64, maxGas uint64) [][]byte {
	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

	var (
		totalBytes uint64
		totalGas   uint64
		result     [][]byte
		nextStart  int
	)

	for idx, tx := range rl.txs {
		if tx == nil {
			// txs may have "holes" (nil) due to txs being invalidated and
			// dropped while they are waiting in the reap list
			nextStart = idx + 1
			continue
		}

		txBytes, err := rl.encodeTx(tx)
		if err != nil {
			nextStart = idx + 1
			continue
		}

		txSize := uint64(len(txBytes))
		txGas := tx.Gas()

		// Check if adding this tx would exceed limits
		if (maxBytes > 0 && totalBytes+txSize > maxBytes) || (maxGas > 0 && totalGas+txGas > maxGas) {
			break
		}

		result = append(result, txBytes)
		totalBytes += txSize
		totalGas += txGas
		nextStart = idx + 1
	}

	if nextStart >= len(rl.txs) {
		rl.txs = types.Transactions{}
	} else {
		// In order to remove the txs that were returned from reap, we can simply
		// reslice the list since all removed txs were from the start, and we saved
		// where the next set of valid txs start in nextStart.
		//
		// Also compact away any nil values from the new slice.
		rl.txs = slices.DeleteFunc(rl.txs[nextStart:], func(tx *types.Transaction) bool {
			return tx == nil
		})
	}

	// rebuild the lookup
	rl.txLookup = make(map[common.Hash]int)
	for i, tx := range rl.txs {
		rl.txLookup[tx.Hash()] = i
	}

	return result
}

// Push inserts a tx to the back of the reap list as the "newest" transaction
// (last to be returned if Reap was called now).
func (rl *reapList) Push(tx *ethtypes.Transaction) {
	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

	rl.txs = append(rl.txs, tx)
	rl.txLookup[tx.Hash()] = len(rl.txs) - 1
}

// Drop removes an individual tx from the reap list. If the tx is not in the
// list, no changes are made. This should only be called when a tx that was
// previously validated, becomes invalid.
func (rl *reapList) Drop(tx *ethtypes.Transaction) {
	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

	idx, ok := rl.txLookup[tx.Hash()]
	if !ok {
		return
	}
	if idx < 0 || idx >= len(rl.txs) {
		return
	}

	rl.txs[idx] = nil
}
