// Package tracer composes geth tracing hooks with application-level tracers.
//
// Applications register one or more factories while wiring the keeper:
//
//	evmKeeper.SetApplicationTracerFactories(
//		application.TxTraceFactory,
//		myPolicyTracerFactory,
//	)
//
// When tracing is configured, each top-level execution receives a new Manager.
// Nested CallEVM calls reuse that manager, while CallEVMViewWithData starts
// an isolated lifecycle.
// A post-transaction hook can read the built-in collector with:
//
//	observations, err := application.GetTxTrace(
//		ctx, uint64(receipt.TransactionIndex),
//	)
//	if err != nil {
//		return err
//	}
//	// Inspect observation kinds, call relationships, and rollback status.
package tracer
