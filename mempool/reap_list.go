package mempool

import (
	"sync"

	"github.com/ethereum/go-ethereum/core/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type reapList struct {
	txs      types.Transactions
	txsLock  sync.RWMutex
	encodeTx func(tx *ethtypes.Transaction) ([]byte, error)
}

func NewReapList(encodeTx func(tx *ethtypes.Transaction) ([]byte, error)) *reapList {
	return &reapList{encodeTx: encodeTx}
}

// Reap returns a list of the oldest to newest transactions bytes from the reap
// list until either maxBytes or maxGas is reached for the list of transactions
// being returned. If maxBytes and maxGas are both 0 all txs will be returned.
//
// If encoding fails for a tx, it is removed from the reap list and is not
// returned.
func (rl *reapList) Reap(maxBytes uint64, maxGas uint64) [][]byte {
	rl.txsLock.RLock()
	var (
		totalBytes uint64
		totalGas   uint64
		result     [][]byte
		nextStart  int
	)

	for idx, tx := range rl.txs {
		nextStart = idx + 1

		txBytes, err := rl.encodeTx(tx)
		if err != nil {
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
	}
	rl.txsLock.RUnlock()

	// In order to remove the txs that were returned from reap, we can simply
	// reslice the list since all removed txs were from the start, and we saved
	// where the next set of valid txs start in nextStart
	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()
	if nextStart == len(rl.txs)-1 {
		rl.txs = types.Transactions{}
	}
	rl.txs = rl.txs[nextStart:len(rl.txs)]

	return result
}

// Push inserts a tx to the back of the reap list as the "newest" transaction
// (last to be returned if Reap was called now).
func (rl *reapList) Push(tx *ethtypes.Transaction) {
	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()
	rl.txs = append(rl.txs, tx)
}
