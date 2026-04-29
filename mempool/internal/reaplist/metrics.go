package reaplist

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("github.com/cosmos/evm/mempool/internal/reaplist")

// reapListMetrics are reapListMetrics specific to the reaplist
type reapListMetrics struct {
	numTxs      metric.Int64Gauge
	numIndexTxs metric.Int64Gauge
	pushedTxs   metric.Int64Counter
	droppedTxs  metric.Int64Counter
	evictedTxs  metric.Int64Counter
}

var metrics = reapListMetrics{}

type txType string

const (
	evmType    txType = "evm"
	cosmosType txType = "cosmos"
)

// TxPushed records that a new tx was pushed into the reap list with type
// txType
func (rlm *reapListMetrics) TxPushed(txType txType) {
	attributes := attribute.NewSet(attribute.String("tx_type", string(txType)))
	rlm.pushedTxs.Add(context.Background(), 1, metric.WithAttributeSet(attributes))
}

// TxDropped records that a new tx was dropped from the reap list with type
// txType
func (rlm *reapListMetrics) TxDropped(txType txType) {
	attributes := attribute.NewSet(attribute.String("tx_type", string(txType)))
	rlm.droppedTxs.Add(context.Background(), 1, metric.WithAttributeSet(attributes))
}

// TxEvicted records that a tx was evicted from the reap list with the given
// reason (e.g. "oversized_bytes", "oversized_gas", "cap_full").
func (rlm *reapListMetrics) TxEvicted(reason string) {
	attributes := attribute.NewSet(attribute.String("reason", reason))
	rlm.evictedTxs.Add(context.Background(), 1, metric.WithAttributeSet(attributes))
}

// RecordNumTxs records the number of entires in txs
func (rlm *reapListMetrics) RecordNumTxs(txs []*txWithHash) {
	rlm.numTxs.Record(context.Background(), int64(len(txs)))
}

// RecordNumTxs records the number of entries in index
func (rlm *reapListMetrics) RecordNumIndexTxs(index map[string]int) {
	rlm.numIndexTxs.Record(context.Background(), int64(len(index)))
}

// initialize all metrics or panic
func init() {
	var err error
	metrics.numTxs, err = meter.Int64Gauge(
		"reap_list.num_txs",
		metric.WithDescription("Number of populated entries currently in the reap list (may be a nil tombstone)"),
	)
	if err != nil {
		panic(err)
	}

	metrics.numIndexTxs, err = meter.Int64Gauge(
		"reap_list.num_index_txs",
		metric.WithDescription("Number of transactions currently in the reap list's index"),
	)
	if err != nil {
		panic(err)
	}

	metrics.pushedTxs, err = meter.Int64Counter(
		"reap_list.pushed_txs",
		metric.WithDescription("Total number of transactions that have been pushed into the reap list"),
	)
	if err != nil {
		panic(err)
	}

	metrics.droppedTxs, err = meter.Int64Counter(
		"reap_list.dropped_txs",
		metric.WithDescription("Total number of transactions that have been dropped from the reap list"),
	)
	if err != nil {
		panic(err)
	}

	metrics.evictedTxs, err = meter.Int64Counter(
		"reap_list.evicted_txs",
		metric.WithDescription("Total number of transactions evicted from the reap list, by reason (oversized_bytes, oversized_gas, cap_full)"),
	)
	if err != nil {
		panic(err)
	}
}
