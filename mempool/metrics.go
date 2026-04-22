package mempool

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// mempoolMetrics is a wrapper around all mempool metrics plus mempool
// subcomponents and their metrics
type mempoolMetrics struct {
	reapList reapListMetrics
}

// metrics is a package local singleton, all mempool components that need
// metrics should access them via this metrics struct
var metrics = mempoolMetrics{}

// reapListMetrics are metrics specific to the reap list component
type reapListMetrics struct {
	numTxs      metric.Int64Gauge
	numIndexTxs metric.Int64Gauge
	pushedTxs   metric.Int64Counter
	droppedTxs  metric.Int64Counter
}

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
	metrics.reapList.numTxs, err = meter.Int64Gauge(
		"mempool.reap_list.num_txs",
		metric.WithDescription("Number of populated entries currently in the reap list (may be a nil tombstone)"),
	)
	if err != nil {
		panic(err)
	}

	metrics.reapList.numIndexTxs, err = meter.Int64Gauge(
		"mempool.reap_list.num_index_txs",
		metric.WithDescription("Number of transactions currently in the reap list's index"),
	)
	if err != nil {
		panic(err)
	}

	metrics.reapList.pushedTxs, err = meter.Int64Counter(
		"mempool.reap_list.pushed_txs",
		metric.WithDescription("Total number of transactions that have been pushed into the reap list"),
	)
	if err != nil {
		panic(err)
	}

	metrics.reapList.droppedTxs, err = meter.Int64Counter(
		"mempool.reap_list.dropped_txs",
		metric.WithDescription("Total number of transactions that have been dropped from the reap list"),
	)
	if err != nil {
		panic(err)
	}
}
