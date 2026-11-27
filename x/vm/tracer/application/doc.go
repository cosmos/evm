// Package application provides an optional ordered execution journal for calls,
// native transfers, ERC-20 call requests, logs, and balance changes.
//
// Register the collector while wiring the application:
//
//	evmKeeper.SetApplicationTracerFactories(application.TxTraceFactory)
//
// Its result is available from the execution context in PostTxProcessing:
//
//	observations, err := application.GetTxTrace(
//		ctx, uint64(receipt.TransactionIndex),
//	)
package application
