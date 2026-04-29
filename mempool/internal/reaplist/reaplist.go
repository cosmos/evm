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

// Unbounded disables the capacity check.
const Unbounded = 0

// ErrReapListFull is returned by Push* when the reap list is at capacity.
var ErrReapListFull = errors.New("reap list full")

const (
	reasonOversizedBytes = "oversized_bytes"
	reasonOversizedGas   = "oversized_gas"
	reasonOversizedBoth  = "oversized_both"
	reasonCapFull        = "cap_full"
)

// TxKind identifies which sub-pool owns a tx so the drop callback can route
// the eviction without re-decoding bytes.
type TxKind int

const (
	KindEVM TxKind = iota
	KindCosmos
)

// DropCallback is invoked after Reap evicts a tx so the owning sub-pool can
// drop it too. cosmosTx is set for KindCosmos only; KindEVM removes via hash.
// Invoked outside the reap list lock so callbacks may re-enter the reap list.
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
	// cosmosTx is set for KindCosmos only; the cosmos pool's Remove takes sdk.Tx.
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

	// maxSize bounds the number of live entries. Unbounded disables the cap.
	maxSize int

	// liveCount is the count of non-nil entries in txs.
	liveCount int

	// dropCallback notifies the owning sub-pool when Reap evicts a tx. May be nil.
	dropCallback DropCallback
}

// New constructs a ReapList. Pass Unbounded to disable the cap; dropCallback may be nil.
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

type pendingDrop struct {
	hash string
	kind TxKind
	tx   sdk.Tx
}

// Reap returns a list of the oldest to newest transactions bytes from the reap
// list until either maxBytes or maxGas is reached for the list of transactions
// being returned. If maxBytes and maxGas are both 0 all txs will be returned.
//
// Txs that exceed maxBytes or maxGas standalone are evicted (they can never fit
// in any block) and the owning sub-pool is notified via dropCallback.
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

		// Standalone-oversized: evict so it can't wedge the head of the list.
		oversizedBytes := maxBytes > 0 && txSize > maxBytes
		oversizedGas := maxGas > 0 && txGas > maxGas
		if oversizedBytes || oversizedGas {
			var reason string
			switch {
			case oversizedBytes && oversizedGas:
				reason = reasonOversizedBoth
			case oversizedBytes:
				reason = reasonOversizedBytes
			default:
				reason = reasonOversizedGas
			}
			rl.evict(idx, reason)
			drops = append(drops, pendingDrop{hash: tx.hash, kind: tx.kind, tx: tx.cosmosTx})
			nextStart = idx + 1
			continue
		}

		// Over the remaining budget; tx fits in some block, just not this reap.
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

	// Post-compaction txs has no nils, so liveCount == len(txs).
	rl.liveCount = len(rl.txs)

	rl.txsLock.Unlock()

	// Callbacks fire outside the lock so they may safely re-enter the reap list.
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
	rl.liveCount++

	metrics.RecordNumTxs(rl.txs)
	metrics.RecordNumIndexTxs(rl.txIndex)
}

// atCapacityLocked reports whether the reap list is at its configured cap.
//
// NOTE: this is not thread safe, callers should be holding the txsLock.
func (rl *ReapList) atCapacityLocked() bool {
	if rl.maxSize == Unbounded {
		return false
	}
	return rl.liveCount >= rl.maxSize
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
	rl.liveCount--
	// NOTE: Not updating numTxs metric here since that reports the size of the
	// reap list **including** tombstones. We will update numTxs when the
	// tombstone is removed via the next `Reap` call.
	return true
}

// evict tombstones the slot at idx and removes the hash from the index.
// Caller must invoke any associated dropCallback AFTER the lock is released.
//
// NOTE: this is not thread safe, callers should be holding the txsLock.
func (rl *ReapList) evict(idx int, reason string) {
	tx := rl.txs[idx]
	if tx == nil {
		return
	}
	delete(rl.txIndex, tx.hash)
	rl.txs[idx] = nil
	rl.liveCount--
	metrics.TxEvicted(reason)
}

func cosmosHash(txBytes []byte) string {
	return fmt.Sprintf("%X", tmhash.Sum(txBytes))
}
