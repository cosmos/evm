package mempool

import (
	"fmt"
	"slices"
	"sync"

	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type EVMCosmosTxEncoder interface {
	EncodeEVMTx(tx *ethtypes.Transaction) ([]byte, error)
	EncodeCosmosTx(tx sdk.Tx) ([]byte, error)
}

type txWithHash struct {
	bytes []byte
	hash  string
	gas   uint64
}

type ReapList struct {
	// txs is a list of transactions and their respective hashes
	txs []*txWithHash

	// txIndex is a map of tx hashes to what index that tx is stored in inside
	// of txs. This serves a dual purpose of allowing for fast drops from txs
	// without iteration, and guarding txs from being added to the ReapList
	// twice before they are explicitly dropped.
	txIndex map[string]int

	// txsLock protects txLookup and txs.
	txsLock sync.RWMutex

	// txEncoder encodes evm and cosmos txs to bytes.
	txEncoder EVMCosmosTxEncoder
}

func NewReapList(txEncoder EVMCosmosTxEncoder) *ReapList {
	return &ReapList{
		txEncoder: txEncoder,
		txIndex:   make(map[string]int),
	}
}

// Reap returns a list of the oldest to newest transactions bytes from the reap
// list until either maxBytes or maxGas is reached for the list of transactions
// being returned. If maxBytes and maxGas are both 0 all txs will be returned.
func (rl *ReapList) Reap(maxBytes uint64, maxGas uint64) [][]byte {
	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

	var (
		totalBytes uint64
		totalGas   uint64
		result     [][]byte
		nextStart  int
		removed    []string
	)

	for idx, tx := range rl.txs {
		if tx == nil {
			// txs may have "holes" (nil) due to txs being invalidated and
			// dropped while they are waiting in the reap list
			nextStart = idx + 1
			continue
		}

		txSize := uint64(len(tx.bytes))
		txGas := tx.gas

		// Check if adding this tx would exceed limits
		if (maxBytes > 0 && totalBytes+txSize > maxBytes) || (maxGas > 0 && totalGas+txGas > maxGas) {
			break
		}

		result = append(result, tx.bytes)
		removed = append(removed, tx.hash)
		totalBytes += txSize
		totalGas += txGas
		nextStart = idx + 1
	}

	if nextStart >= len(rl.txs) {
		rl.txs = []*txWithHash{}
	} else {
		// In order to remove the txs that were returned from reap, we can simply
		// reslice the list since all removed txs were from the start, and we saved
		// where the next set of valid txs start in nextStart.
		//
		// Also compact away any nil values from the new slice.
		rl.txs = slices.DeleteFunc(rl.txs[nextStart:], func(tx *txWithHash) bool {
			return tx == nil
		})
	}

	// rebuild the index
	rl.txIndex = make(map[string]int)
	for i, tx := range rl.txs {
		rl.txIndex[tx.hash] = i
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

// PushEVMTx enqueues an EVM tx into the reap list.
func (rl *ReapList) PushEVMTx(tx *ethtypes.Transaction) error {
	txBytes, err := rl.txEncoder.EncodeEVMTx(tx)
	if err != nil {
		return fmt.Errorf("encoding evm tx to bytes: %w", err)
	}

	hash := tx.Hash()
	rl.push(hash.String(), txBytes, tx.Gas())
	return nil
}

// PushCosmosTx enqueues a cosmos tx into the reap list.
func (rl *ReapList) PushCosmosTx(tx sdk.Tx) error {
	txBytes, err := rl.txEncoder.EncodeCosmosTx(tx)
	if err != nil {
		return fmt.Errorf("encoding evm tx to bytes: %w", err)
	}

	var gas uint64
	if feeTx, ok := tx.(sdk.FeeTx); ok {
		gas = feeTx.GetGas()
	}

	hash := fmt.Sprintf("%X", tmhash.Sum(txBytes))
	rl.push(hash, txBytes, gas)
	return nil
}

// push inserts a tx to the back of the reap list as the "newest" transaction
// (last to be returned if Reap was called now).
func (rl *ReapList) push(hash string, tx []byte, gas uint64) {
	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

	if _, ok := rl.txIndex[hash]; ok {
		return
	}

	rl.txs = append(rl.txs, &txWithHash{tx, hash, gas})
	rl.txIndex[hash] = len(rl.txs) - 1
}

// DropEVMTx removes an EVM tx from the ReapList. This tx may or may not have
// already been reaped. This should only be called when a tx that was
// previously validated, becomes invalid.
func (rl *ReapList) DropEVMTx(tx *ethtypes.Transaction) {
	rl.drop(tx.Hash().String())
}

// DropCosmosTx removes a Cosmos tx from the ReapList. This tx may or may not
// have already been reaped. This should only be called when a tx that was
// previously validated, becomes invalid.
func (rl *ReapList) DropCosmosTx(tx sdk.Tx) {
	txBytes, err := rl.txEncoder.EncodeCosmosTx(tx)
	if err != nil {
		return
	}
	rl.drop(fmt.Sprintf("%X", tmhash.Sum(txBytes)))
}

// drop removes an individual tx from the reap list. If the tx is not in the
// list, no changes are made.
func (rl *ReapList) drop(hash string) {
	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

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
