package application

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	vmtracer "github.com/cosmos/evm/x/vm/tracer"
)

type txTraceContextKey struct{}

type txTraceContext struct {
	txIndex   uint64
	collector Collector
}

// TxTraceFactory creates a standard transaction-wide collector. Applications
// can pass this function to keeper.SetApplicationTracerFactories to opt in to
// call, asset movement, balance and log collection.
func TxTraceFactory(ctx sdk.Context, execution vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer) {
	collector := NewCollector()
	return WithCollector(ctx, execution.TxIndex, collector), collector
}

// WithCollector stores a collector for the current EVM transaction in ctx. The
// value is available only through the returned execution context and contexts
// derived from it; committing an SDK cache context does not copy it to the
// parent context.
func WithCollector(ctx sdk.Context, txIndex uint64, collector Collector) sdk.Context {
	return ctx.WithValue(txTraceContextKey{}, txTraceContext{
		txIndex:   txIndex,
		collector: collector,
	})
}

// GetTxTrace returns the ordered journal and latched error for txIndex.
// The error must be checked even when an EVM call caught a child revert.
func GetTxTrace(ctx sdk.Context, txIndex uint64) ([]vmtracer.Observation, error) {
	trace, ok := ctx.Value(txTraceContextKey{}).(txTraceContext)
	if !ok || trace.txIndex != txIndex || trace.collector == nil {
		return nil, nil
	}
	return trace.collector.Observations(), trace.collector.Error()
}
