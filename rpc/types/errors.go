package types

import "errors"

var (
	ErrProfilingDisabled = errors.New("profiling disabled in the debug namespace")
	// ErrMempoolDisabled is returned on tx submission when the node runs without
	// the app-side EVM mempool (mempool.max-txs = -1).
	ErrMempoolDisabled = errors.New("EVM mempool is disabled: tx submission over JSON-RPC is unavailable")
)
