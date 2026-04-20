package mempool

import (
	"context"
	"fmt"
	"slices"
	"sync"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/cometbft/cometbft/crypto/tmhash"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	numTxs      metric.Int64Gauge
	numIndexTxs metric.Int64Gauge
	pushedTxs   metric.Int64Counter
	droppedTxs  metric.Int64Counter
)

func init() {
	var err error
	numTxs, err = meter.Int64Gauge(
		"mempool.reap_list.num_txs",
		metric.WithDescription("Number of transactions currently in the reap list"),
	)
	if err != nil {
		panic(err)
	}

	numIndexTxs, err = meter.Int64Gauge(
		"mempool.reap_list.num_index_txs",
		metric.WithDescription("Number of transactions currently in the reap list's index"),
	)
	if err != nil {
		panic(err)
	}

	pushedTxs, err = meter.Int64Counter(
		"mempool.reap_list.pushed_txs",
		metric.WithDescription("Total number of transactions that have been pushed into the reap list"),
	)
	if err != nil {
		panic(err)
	}

	droppedTxs, err = meter.Int64Counter(
		"mempool.reap_list.dropped_txs",
		metric.WithDescription("Total number of transactions that have been dropped from the reap list"),
	)
	if err != nil {
		panic(err)
	}
}

type EVMCosmosTxEncoder interface {
	EVMTx(tx *ethtypes.Transaction) ([]byte, error)
	CosmosTx(tx sdk.Tx) ([]byte, error)
}

type txWithHash struct {
	bytes []byte
	hash  string
	gas   uint64
}

type ReapList struct {
	// txs is a list of transactions and their respective hashes
	// NOTE: this currently has unbound size
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
	numTxs.Record(context.Background(), int64(len(rl.txs)))

	// rebuild the index since txs may have shifted indices
	for i, tx := range rl.txs {
		if _, ok := rl.txIndex[tx.hash]; !ok {
			panic("tx that was not reaped is not in the tx index, this should not happen")
		}
		rl.txIndex[tx.hash] = i
	}
	numIndexTxs.Record(context.Background(), int64(len(rl.txIndex)))

	return result
}

// PushEVMTx enqueues an EVM tx into the reap list.
func (rl *ReapList) PushEVMTx(tx *ethtypes.Transaction) error {
	hash := tx.Hash().String()
	if rl.exists(hash) {
		return nil
	}

	txBytes, err := rl.txEncoder.EVMTx(tx)
	if err != nil {
		return fmt.Errorf("encoding evm tx to bytes: %w", err)
	}

	rl.push(hash, txBytes, tx.Gas())

	attributes := attribute.NewSet(attribute.String("tx_type", "evm"))
	pushedTxs.Add(context.Background(), 1, metric.WithAttributeSet(attributes))
	return nil
}

// PushCosmosTx enqueues a cosmos tx into the reap list.
func (rl *ReapList) PushCosmosTx(tx sdk.Tx) error {
	txBytes, err := rl.txEncoder.CosmosTx(tx)
	if err != nil {
		return fmt.Errorf("encoding cosmos tx to bytes: %w", err)
	}

	hash := cosmosHash(txBytes)
	if rl.exists(hash) {
		return nil
	}

	var gas uint64
	if feeTx, ok := tx.(sdk.FeeTx); ok {
		gas = feeTx.GetGas()
	} else {
		return fmt.Errorf("error getting tx gas: cosmos tx must implement sdk.FeeTx")
	}

	rl.push(hash, txBytes, gas)

	attributes := attribute.NewSet(attribute.String("tx_type", "cosmos"))
	pushedTxs.Add(context.Background(), 1, metric.WithAttributeSet(attributes))
	return nil
}

// push inserts a tx to the back of the reap list as the "newest" transaction
// (last to be returned if Reap was called now). push assumes that a tx is not
// already in the ReapList, this should be checked via exists.
func (rl *ReapList) push(hash string, tx []byte, gas uint64) {
	rl.txsLock.Lock()
	defer rl.txsLock.Unlock()

	rl.txs = append(rl.txs, &txWithHash{tx, hash, gas})
	rl.txIndex[hash] = len(rl.txs) - 1

	numTxs.Record(context.Background(), int64(len(rl.txs)))
	numIndexTxs.Record(context.Background(), int64(len(rl.txIndex)))
}

// exists returns true if a hash is in the index, false otherwise.
func (rl *ReapList) exists(hash string) bool {
	rl.txsLock.RLock()
	defer rl.txsLock.RUnlock()

	_, ok := rl.txIndex[hash]
	return ok
}

// DropEVMTx removes an EVM tx from the ReapList. This tx may or may not have
// already been reaped. This should only be called when a tx that was
// previously validated, becomes invalid.
func (rl *ReapList) DropEVMTx(tx *ethtypes.Transaction) {
	rl.drop(tx.Hash().String())

	attributes := attribute.NewSet(attribute.String("tx_type", "evm"))
	droppedTxs.Add(context.Background(), 1, metric.WithAttributeSet(attributes))
}

// DropCosmosTx removes a Cosmos tx from the ReapList. This tx may or may not
// have already been reaped. This should only be called when a tx that was
// previously validated, becomes invalid.
func (rl *ReapList) DropCosmosTx(tx sdk.Tx) {
	txBytes, err := rl.txEncoder.CosmosTx(tx)
	if err != nil {
		return
	}
	rl.drop(cosmosHash(txBytes))

	attributes := attribute.NewSet(attribute.String("tx_type", "cosmos"))
	droppedTxs.Add(context.Background(), 1, metric.WithAttributeSet(attributes))
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
	numIndexTxs.Record(context.Background(), int64(len(rl.txIndex)))

	if idx < 0 || idx >= len(rl.txs) {
		return
	}

	rl.txs[idx] = nil
}

func cosmosHash(txBytes []byte) string {
	return fmt.Sprintf("%X", tmhash.Sum(txBytes))
}
