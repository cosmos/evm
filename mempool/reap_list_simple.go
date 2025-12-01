package mempool

import (
	"sync"

	"github.com/ethereum/go-ethereum/core/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type reapListSimple struct {
	list     types.Transactions
	listLock sync.RWMutex
}

func (rl *reapListSimple) Reap(maxBytes uint64, maxGas uint64, encode func(tx *ethtypes.Transaction) ([]byte, error)) [][]byte {
	rl.listLock.RLock()
	var (
		totalBytes uint64
		totalGas   uint64
		result     [][]byte
		nextStart  int
	)

	for idx, tx := range rl.list {
		nextStart = idx + 1

		txBytes, err := encode(tx)
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
	rl.listLock.RUnlock()

	// In order to remove the txs that were returned from reap, we can simply
	// reslice the list since all removed txs were from the start, and we saved
	// where the next set of valid txs start in nextStart
	rl.listLock.Lock()
	defer rl.listLock.Unlock()
	if nextStart == len(rl.list)-1 {
		rl.list = types.Transactions{}
	}
	rl.list = rl.list[nextStart:len(rl.list)]

	return result
}

func (rl *reapListSimple) Insert(tx *ethtypes.Transaction) {
	rl.listLock.Lock()
	defer rl.listLock.Unlock()
	rl.list = append(rl.list, tx)
}
