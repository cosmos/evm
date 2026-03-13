package mempool

import (
	"fmt"
	"sync"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cometbft/cometbft/crypto/tmhash"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type EVMCosmosTxEncoder interface {
	EVMTx(tx *ethtypes.Transaction) ([]byte, error)
	CosmosTx(tx sdk.Tx) ([]byte, error)
}

type txWithHash struct {
	bytes []byte
	hash  string
	gas   uint64
}

const reapListCompactThreshold = 1024

type ReapList struct {
	// txs is a list of transactions and their respective hashes.
	// Entries before head have already been reaped or compacted away.
	// NOTE: this currently has unbound size.
	txs []*txWithHash

	// head is the first candidate index in txs that has not yet been reaped.
	head int

	// txIndex maps tx hashes to their absolute index in txs.
	// Reaped txs are kept with an index of -1 until they are dropped.
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
// list until either maxBytes or maxGas is reached. Uses a head offset so only
// the reaped prefix is traversed (O(k) for k reaped txs) with amortized
// compaction of the backing slice.
func (rl *ReapList) Reap(maxBytes uint64, maxGas uint64) [][]byte {
	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

	var (
		totalBytes uint64
		totalGas   uint64
		result     [][]byte
		nextHead   = rl.head
	)

	for idx := rl.head; idx < len(rl.txs); idx++ {
		tx := rl.txs[idx]
		if tx == nil {
			// txs may have "holes" (nil) due to txs being invalidated and
			// dropped while they are waiting in the reap list.
			nextHead = idx + 1
			continue
		}

		txSize := uint64(len(tx.bytes))
		txGas := tx.gas

		// Check if adding this tx would exceed limits.
		if (maxBytes > 0 && totalBytes+txSize > maxBytes) || (maxGas > 0 && totalGas+txGas > maxGas) {
			break
		}

		result = append(result, tx.bytes)
		totalBytes += txSize
		totalGas += txGas
		nextHead = idx + 1

		// Keep reaped txs in txIndex with sentinel -1 so callers cannot
		// re-add them until they are explicitly dropped.
		if _, ok := rl.txIndex[tx.hash]; !ok {
			panic("removed a tx that was not in the tx index, this should not happen")
		}
		rl.txIndex[tx.hash] = -1
	}

	rl.head = nextHead
	rl.maybeCompactLocked()

	return result
}

// PushEVMTx enqueues an EVM tx into the reap list.
func (rl *ReapList) PushEVMTx(tx *ethtypes.Transaction) error {
	hash := tx.Hash().String()

	txBytes, err := rl.txEncoder.EVMTx(tx)
	if err != nil {
		return fmt.Errorf("encoding evm tx to bytes: %w", err)
	}

	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

	rl.pushLocked(hash, txBytes, tx.Gas())
	return nil
}

// PushCosmosTx enqueues a cosmos tx into the reap list.
func (rl *ReapList) PushCosmosTx(tx sdk.Tx) error {
	txBytes, err := rl.txEncoder.CosmosTx(tx)
	if err != nil {
		return fmt.Errorf("encoding cosmos tx to bytes: %w", err)
	}

	var gas uint64
	if feeTx, ok := tx.(sdk.FeeTx); ok {
		gas = feeTx.GetGas()
	} else {
		return fmt.Errorf("error getting tx gas: cosmos tx must implement sdk.FeeTx")
	}

	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

	rl.pushLocked(cosmosHash(txBytes), txBytes, gas)
	return nil
}

// pushLocked inserts a tx to the back of the reap list. Deduplicates by hash
// under the already-held write lock.
func (rl *ReapList) pushLocked(hash string, tx []byte, gas uint64) {
	if _, ok := rl.txIndex[hash]; ok {
		return
	}

	if rl.head >= len(rl.txs) && len(rl.txs) > 0 {
		// All active entries have been consumed; reuse the backing array.
		rl.txs = rl.txs[:0]
		rl.head = 0
	}

	rl.txs = append(rl.txs, &txWithHash{bytes: tx, hash: hash, gas: gas})
	rl.txIndex[hash] = len(rl.txs) - 1
}

// maybeCompactLocked compacts the backing slice when enough of the prefix has
// been consumed, keeping Reap amortized O(k).
func (rl *ReapList) maybeCompactLocked() {
	if rl.head == 0 {
		return
	}

	if rl.head >= len(rl.txs) {
		rl.txs = rl.txs[:0]
		rl.head = 0
		return
	}

	// Compact when we have consumed a meaningful prefix.
	if rl.head < reapListCompactThreshold && rl.head*2 < len(rl.txs) {
		return
	}

	compacted := make([]*txWithHash, 0, len(rl.txs)-rl.head)
	for idx := rl.head; idx < len(rl.txs); idx++ {
		tx := rl.txs[idx]
		if tx == nil {
			continue
		}
		if _, ok := rl.txIndex[tx.hash]; !ok {
			panic("tx that was not reaped is not in the tx index, this should not happen")
		}
		rl.txIndex[tx.hash] = len(compacted)
		compacted = append(compacted, tx)
	}

	rl.txs = compacted
	rl.head = 0
}

// DropEVMTx removes an EVM tx from the ReapList.
func (rl *ReapList) DropEVMTx(tx *ethtypes.Transaction) {
	rl.drop(tx.Hash().String())
}

// DropCosmosTx removes a Cosmos tx from the ReapList.
func (rl *ReapList) DropCosmosTx(tx sdk.Tx) {
	txBytes, err := rl.txEncoder.CosmosTx(tx)
	if err != nil {
		return
	}
	rl.drop(cosmosHash(txBytes))
}

// drop removes an individual tx from the reap list.
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

func cosmosHash(txBytes []byte) string {
	return fmt.Sprintf("%X", tmhash.Sum(txBytes))
}
