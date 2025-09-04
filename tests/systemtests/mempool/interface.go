package mempool

import (
	"math/big"
	"time"
)

type TestSuite interface {
	// Tx
	SendTx(nodeID string, accID string, nonce uint64, gasPrice *big.Int, gasTipCap *big.Int) (string, error)

	// Query
	WaitForCommit(nodeID string, txHash string, timeout time.Duration) error
	BaseFee() *big.Int
	BaseFeeX2() *big.Int

	// Expectation
	OnlyEthTxs() bool

	ExpPendingTxs() []string
	ExpPendingTx(idx int) string
	SetExpPendingTxs(txs ...string)

	ExpQueuedTxs() []string
	ExpQueuedTx(idx int) string
	SetExpQueuedTxs(txs ...string)

	PromoteExpTxs(count int)
}
