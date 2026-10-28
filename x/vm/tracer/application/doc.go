// Package application provides an optional transaction-wide collector for
// call touches, native value transfers, and standard ERC-20 transfer calls.
//
// Register the collector while wiring the application:
//
//	evmKeeper.SetApplicationTracerFactories(application.TxTraceFactory)
//
// Its result is available from the execution context in PostTxProcessing:
//
//	touches, transfers, erc20Transfers := application.GetTxTrace(
//		ctx, uint64(receipt.TransactionIndex),
//	)
package application
