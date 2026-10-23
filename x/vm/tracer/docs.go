// Package tracer composes geth tracing hooks with application-level tracers.
//
// Applications register one or more factories while wiring the keeper:
//
//	evmKeeper.SetApplicationTracerFactories(
//		application.TxTraceFactory,
//		myPolicyTracerFactory,
//	)
//
// Each top-level EVM execution receives a new Manager. Nested CallEVM calls
// reuse that manager, while CallEVMViewWithData starts an isolated lifecycle.
// A post-transaction hook can read the built-in collector with:
//
//	touches, transfers, erc20Transfers := application.GetTxTrace(
//		ctx, uint64(receipt.TransactionIndex),
//	)
package tracer
