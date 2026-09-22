package application

import (
	vmtracer "github.com/cosmos/evm/x/vm/tracer"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type txTraceContextKey struct{}

type txTraceContext struct {
	txIndex   uint64
	collector Collector
}

// TxTraceFactory creates a standard transaction-wide collector. Applications
// can pass this function to keeper.SetApplicationTracerFactories to opt in to
// call-touch, native-transfer, and standard ERC20 transfer-call collection.
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

// GetTxTrace returns call touches, native value transfers, and standard ERC20
// transfer calls collected for txIndex in the current EVM execution context.
// It returns nil slices when the context has no trace or belongs to a different
// transaction.
func GetTxTrace(ctx sdk.Context, txIndex uint64) (touches []TxCallTouch, transfers []TxValueTransfer, erc20Transfers []TxERC20Transfer) {
	trace, ok := ctx.Value(txTraceContextKey{}).(txTraceContext)
	if !ok || trace.txIndex != txIndex || trace.collector == nil {
		return nil, nil, nil
	}
	return trace.collector.Touches(), trace.collector.Transfers(), trace.collector.ERC20Transfers()
}
