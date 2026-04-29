package reaplist

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cometbft/cometbft/crypto/tmhash"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Unbounded is the sentinel value for ReapList.maxSize that disables the
// capacity check, restoring legacy unbounded behavior.
const Unbounded = 0

// ErrReapListFull is returned by Push* when the reap list is at capacity.
var ErrReapListFull = errors.New("reap list full")

// Eviction reasons used by metrics and surfaced via the drop callback.
const (
	reasonOversizedBytes = "oversized_bytes"
	reasonOversizedGas   = "oversized_gas"
	reasonCapFull        = "cap_full"
)

// TxKind identifies which sub-pool owns a tx so the drop callback can route
// the eviction without re-decoding bytes.
type TxKind int

const (
	KindEVM TxKind = iota
	KindCosmos
)

// DropCallback is invoked from Reap when a tx is evicted from the reap list
// (e.g. permanently oversized for any block). The callback should remove the
// tx from its owning sub-pool. It is invoked AFTER the reap list lock has been
// released so callbacks may safely re-enter the reap list.
//
// For KindCosmos evictions, cosmosTx is the original sdk.Tx (populated at
// PushCosmosTx time) so callers can drive the cosmos pool's typed removal API
// without re-decoding bytes. For KindEVM evictions, cosmosTx is nil and
// callers should remove via the hash.
type DropCallback func(hash string, kind TxKind, cosmosTx sdk.Tx)

type EVMCosmosTxEncoder interface {
	EVMTx(tx *ethtypes.Transaction) ([]byte, error)
	CosmosTx(tx sdk.Tx) ([]byte, error)
}

type txWithHash struct {
	bytes []byte
	hash  string
	gas   uint64
	kind  TxKind
	// cosmosTx is populated for KindCosmos entries only. The drop callback
	// requires the original sdk.Tx to remove from the cosmos sub-pool, since
	// the cosmos pool API takes sdk.Tx (not bytes/hash).
	cosmosTx sdk.Tx
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

	// maxSize is the upper bound on len(txs). Tombstones (nil entries) count
	// toward the cap until the next Reap compacts them. If maxSize == Unbounded
	// the cap check is skipped.
	maxSize int

	// dropCallback, if set, is invoked from Reap when a tx is evicted (e.g.
	// permanently oversized) so the owning sub-pool can also drop it. May be
	// nil; callers in tests can pass nil to disable the side effect.
	dropCallback DropCallback
}

// New constructs a ReapList. Pass Unbounded for maxSize to disable the cap.
// dropCallback may be nil.
func New(txEncoder EVMCosmosTxEncoder, maxSize int, dropCallback DropCallback) *ReapList {
	if maxSize < 0 {
		maxSize = Unbounded
	}
	return &ReapList{
		txEncoder:    txEncoder,
		txIndex:      make(map[string]int),
		maxSize:      maxSize,
		dropCallback: dropCallback,
	}
}

// pendingDrop carries a deferred drop callback invocation collected during
// Reap. Callbacks are invoked after the reap list lock is released so they
// may safely re-enter the reap list.
type pendingDrop struct {
	hash string
	kind TxKind
	tx   sdk.Tx
}

// Reap returns a list of the oldest to newest transactions bytes from the reap
// list until either maxBytes or maxGas is reached for the list of transactions
// being returned. If maxBytes and maxGas are both 0 all txs will be returned.
//
// Permanently oversized txs (txSize > maxBytes or txGas > maxGas) are evicted
// from the reap list (and their owning sub-pool, via dropCallback) since they
// can never fit in any block under the current limits.
func (rl *ReapList) Reap(maxBytes uint64, maxGas uint64) [][]byte {
	rl.txsLock.Lock()

	var (
		totalBytes uint64
		totalGas   uint64
		result     [][]byte
		nextStart  int
		drops      []pendingDrop
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

		// Permanently oversized: tx alone exceeds the block limit. Evict so a
		// single oversized tx at the head of the list cannot block subsequent
		// reaps indefinitely.
		oversizedBytes := maxBytes > 0 && txSize > maxBytes
		oversizedGas := maxGas > 0 && txGas > maxGas
		if oversizedBytes || oversizedGas {
			reason := reasonOversizedBytes
			if oversizedGas && !oversizedBytes {
				reason = reasonOversizedGas
			}
			rl.evict(idx, reason)
			drops = append(drops, pendingDrop{hash: tx.hash, kind: tx.kind, tx: tx.cosmosTx})
			// the slot is now nil; advance nextStart so we don't keep
			// re-considering it on subsequent iterations (handled naturally
			// since loop is over a fixed snapshot of rl.txs).
			nextStart = idx + 1
			continue
		}

		// Adding this tx would exceed the remaining budget. Stop here -- the
		// tx itself fits, just not in this reap.
		if (maxBytes > 0 && totalBytes+txSize > maxBytes) || (maxGas > 0 && totalGas+txGas > maxGas) {
			break
		}

		result = append(result, tx.bytes)
		totalBytes += txSize
		totalGas += txGas
		nextStart = idx + 1

		// NOTE: We need to keep the txs that were just reaped in the txIndex, so
		// that it can properly guard against these txs being added to the ReapList
		// again. These txs are likely still in the mempool, and callers may try to
		// add them to the ReapList again, which is not allowed. Removing from the
		// txIndex will only be done during Drop.
		if _, ok := rl.txIndex[tx.hash]; !ok {
			panic("removed a tx that was not in the tx index, this should not happen")
		}
		rl.txIndex[tx.hash] = -1
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
	metrics.RecordNumTxs(rl.txs)

	// rebuild the index since txs may have shifted indices
	for i, tx := range rl.txs {
		if _, ok := rl.txIndex[tx.hash]; !ok {
			panic("tx that was not reaped is not in the tx index, this should not happen")
		}
		rl.txIndex[tx.hash] = i
	}
	metrics.RecordNumIndexTxs(rl.txIndex)

	rl.txsLock.Unlock()

	// Invoke drop callbacks AFTER releasing the lock so the sub-pool's drop
	// path may safely re-enter the reap list (e.g. via DropCosmosTx -> drop).
	if rl.dropCallback != nil {
		for _, d := range drops {
			rl.dropCallback(d.hash, d.kind, d.tx)
		}
	}

	return result
}

// PushEVMTx enqueues an EVM tx into the reap list.
func (rl *ReapList) PushEVMTx(tx *ethtypes.Transaction) error {
	hash := tx.Hash().String()

	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

	if rl.exists(hash) {
		return nil
	}

	if rl.atCapacityLocked() {
		metrics.TxEvicted(reasonCapFull)
		return ErrReapListFull
	}

	txBytes, err := rl.txEncoder.EVMTx(tx)
	if err != nil {
		return fmt.Errorf("encoding evm tx to bytes: %w", err)
	}

	rl.push(&txWithHash{bytes: txBytes, hash: hash, gas: tx.Gas(), kind: KindEVM})

	metrics.TxPushed(evmType)
	return nil
}

// PushCosmosTx enqueues a cosmos tx into the reap list.
func (rl *ReapList) PushCosmosTx(tx sdk.Tx) error {
	txBytes, err := rl.txEncoder.CosmosTx(tx)
	if err != nil {
		return fmt.Errorf("encoding cosmos tx to bytes: %w", err)
	}
	hash := cosmosHash(txBytes)

	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

	if rl.exists(hash) {
		return nil
	}

	if rl.atCapacityLocked() {
		metrics.TxEvicted(reasonCapFull)
		return ErrReapListFull
	}

	var gas uint64
	if feeTx, ok := tx.(sdk.FeeTx); ok {
		gas = feeTx.GetGas()
	} else {
		return fmt.Errorf("error getting tx gas: cosmos tx must implement sdk.FeeTx")
	}

	rl.push(&txWithHash{bytes: txBytes, hash: hash, gas: gas, kind: KindCosmos, cosmosTx: tx})

	metrics.TxPushed(cosmosType)
	return nil
}

// push inserts a tx to the back of the reap list as the "newest" transaction
// (last to be returned if Reap was called now). push assumes that a tx is not
// already in the ReapList, this should be checked via exists.
//
// NOTE: this is not thread safe, callers should be holding the txsLock.
func (rl *ReapList) push(entry *txWithHash) {
	rl.txs = append(rl.txs, entry)
	rl.txIndex[entry.hash] = len(rl.txs) - 1

	metrics.RecordNumTxs(rl.txs)
	metrics.RecordNumIndexTxs(rl.txIndex)
}

// atCapacityLocked reports whether the reap list is at its configured cap.
// Tombstones (nil entries) count toward the cap; they hold memory until the
// next Reap compacts them.
//
// NOTE: this is not thread safe, callers should be holding the txsLock.
func (rl *ReapList) atCapacityLocked() bool {
	if rl.maxSize == Unbounded {
		return false
	}
	return len(rl.txs) >= rl.maxSize
}

// exists returns true if a hash is in the index, false otherwise.
//
// NOTE: this is not thread safe, callers should be holding the txsLock.
func (rl *ReapList) exists(hash string) bool {
	_, ok := rl.txIndex[hash]
	return ok
}

// DropEVMTx removes an EVM tx from the ReapList. This tx may or may not have
// already been reaped. This should only be called when a tx that was
// previously validated, becomes invalid.
func (rl *ReapList) DropEVMTx(tx *ethtypes.Transaction) {
	dropped := rl.drop(tx.Hash().String())

	if dropped {
		metrics.TxDropped(evmType)
	}
}

// DropCosmosTx removes a Cosmos tx from the ReapList. This tx may or may not
// have already been reaped. This should only be called when a tx that was
// previously validated, becomes invalid.
func (rl *ReapList) DropCosmosTx(tx sdk.Tx) {
	txBytes, err := rl.txEncoder.CosmosTx(tx)
	if err != nil {
		return
	}
	dropped := rl.drop(cosmosHash(txBytes))

	if dropped {
		metrics.TxDropped(cosmosType)
	}
}

// drop removes an individual tx from the reap list. If the tx is not in the
// list, no changes are made. Returns true if the tx was dropped, false
// otherwise.
func (rl *ReapList) drop(hash string) bool {
	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

	idx, ok := rl.txIndex[hash]
	if !ok {
		return false
	}
	delete(rl.txIndex, hash)
	metrics.RecordNumIndexTxs(rl.txIndex)

	if idx < 0 || idx >= len(rl.txs) {
		return false
	}

	rl.txs[idx] = nil
	// NOTE: Not updating numTxs metric here since that reports the size of the
	// reap list **including** tombstones. We will update numTxs when the
	// tombstone is removed via the next `Reap` call.
	return true
}

// evict tombstones the slot at idx and removes the hash from the index.
// Increments the eviction metric for the given reason. Caller must hold
// rl.txsLock and must invoke any associated dropCallback AFTER the lock is
// released.
//
// NOTE: this is not thread safe, callers should be holding the txsLock.
func (rl *ReapList) evict(idx int, reason string) {
	tx := rl.txs[idx]
	if tx == nil {
		return
	}
	delete(rl.txIndex, tx.hash)
	rl.txs[idx] = nil
	metrics.TxEvicted(reason)
}

func cosmosHash(txBytes []byte) string {
	return fmt.Sprintf("%X", tmhash.Sum(txBytes))
}
