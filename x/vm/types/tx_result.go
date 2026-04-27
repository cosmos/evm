package types

import (
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
)

// ExceedBlockGasLimitError is the ante-handler error string emitted when the
// block gas meter runs out. The tx fee is already deducted in ante, so the tx
// is still counted toward the eth rank space.
const ExceedBlockGasLimitError = "out of gas in location: block gas meter; gasWanted:"

// TxExceedBlockGasLimit reports whether res failed due to block gas exhaustion.
func TxExceedBlockGasLimit(res *abci.ExecTxResult) bool {
	return strings.Contains(res.Log, ExceedBlockGasLimitError)
}

// TxSucessOrExpectedFailure reports whether a tx participates in the eth rank
// space — success or an expected ExceedBlockGasLimit failure. The indexer and
// RPC backend gate inclusion on this; PatchTxResponses must agree.
func TxSucessOrExpectedFailure(res *abci.ExecTxResult) bool {
	return res.Code == 0 || TxExceedBlockGasLimit(res)
}
