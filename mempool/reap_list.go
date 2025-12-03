package mempool

import (
	"slices"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type ReapList struct {
	// txs is a list of transactions.
	txs ethtypes.Transactions

	// txIndex is a map of tx hashes to what index that tx is stored in inside
	// of txs. This serves a dual purpose of allowing for fast drops from txs
	// without iteration, and guarding txs from being added to the ReapList
	// twice before they are explicitly dropped.
	txIndex map[common.Hash]int

	// txsLock protects txLookup and txs.
	txsLock sync.RWMutex

	// encodeTx encodes a tx to bytes.
	encodeTx func(tx *ethtypes.Transaction) ([]byte, error)
}

func NewReapList(encodeTx func(tx *ethtypes.Transaction) ([]byte, error)) *ReapList {
	return &ReapList{
		encodeTx: encodeTx,
		txIndex:  make(map[common.Hash]int),
	}
}

// Reap returns a list of the oldest to newest transactions bytes from the reap
// list until either maxBytes or maxGas is reached for the list of transactions
// being returned. If maxBytes and maxGas are both 0 all txs will be returned.
//
// If encoding fails for a tx, it is removed from the reap list and is not
// returned.
func (rl *ReapList) Reap(maxBytes uint64, maxGas uint64) [][]byte {
	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

	var (
		totalBytes uint64
		totalGas   uint64
		result     [][]byte
		nextStart  int
		removed    []common.Hash
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
			// while we are not explicitly calling drop to remove this bad tx,
			// it will still be removed since we are setting nextStart to be >
			// the idx of this tx in the list. Once we have collected txs to
			// reap, we will reslice the list to be txs[nextStart:], which will
			// no longer contain this tx.
			nextStart = idx + 1
			removed = append(removed, tx.Hash())
			continue
		}

		txSize := uint64(len(txBytes))
		txGas := tx.Gas()

		// Check if adding this tx would exceed limits
		if (maxBytes > 0 && totalBytes+txSize > maxBytes) || (maxGas > 0 && totalGas+txGas > maxGas) {
			break
		}

		result = append(result, txBytes)
		removed = append(removed, tx.Hash())
		totalBytes += txSize
		totalGas += txGas
		nextStart = idx + 1
	}

	if nextStart >= len(rl.txs) {
		rl.txs = ethtypes.Transactions{}
	} else {
		// In order to remove the txs that were returned from reap, we can simply
		// reslice the list since all removed txs were from the start, and we saved
		// where the next set of valid txs start in nextStart.
		//
		// Also compact away any nil values from the new slice.
		rl.txs = slices.DeleteFunc(rl.txs[nextStart:], func(tx *ethtypes.Transaction) bool {
			return tx == nil
		})
	}

	// rebuild the index
	rl.txIndex = make(map[common.Hash]int)
	for i, tx := range rl.txs {
		rl.txIndex[tx.Hash()] = i
	}

	// NOTE: We need to keep the txs that were just reaped in the txIndex, so
	// that it can properly guard against these txs being added to the ReapList
	// again. These txs are likely still in the mempool, and callers may try to
	// add them to the ReapList again, which is not allowed. Removing from the
	// txIndex will only be done during Drop.
	for _, hash := range removed {
		rl.txIndex[hash] = -1
	}

	return result
}

// Push inserts a tx to the back of the reap list as the "newest" transaction
// (last to be returned if Reap was called now).
func (rl *ReapList) Push(tx *ethtypes.Transaction) {
	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

	if _, ok := rl.txIndex[tx.Hash()]; ok {
		return
	}

	rl.txs = append(rl.txs, tx)
	rl.txIndex[tx.Hash()] = len(rl.txs) - 1
}

// Drop removes an individual tx from the reap list. If the tx is not in the
// list, no changes are made. This should only be called when a tx that was
// previously validated, becomes invalid.
func (rl *ReapList) Drop(tx *ethtypes.Transaction) {
	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

	hash := tx.Hash()
	idx, ok := rl.txIndex[hash]
	if !ok {
		return
	}
	delete(rl.txIndex, hash)

	if idx < 0 || idx >= len(rl.txs) {
		return
	}

	rl.txs[idx] = nil
}
