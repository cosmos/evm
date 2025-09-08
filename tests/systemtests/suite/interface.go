package suite

import (
	"math/big"
	"testing"
	"time"
	// "github.com/cosmos/evm/tests/systemtests/suite"
)

type TestSuite interface {
	// Test Lifecycle
	BeforeEach(t *testing.T)
	AfterEach(t *testing.T)
	JustAfterEach(t *testing.T)

	// Tx
	SendTx(t *testing.T, nodeID string, accID string, nonce uint64, gasPrice *big.Int, gasTipCap *big.Int) (*TxInfo, error)
	SendEthTx(t *testing.T, nodeID string, accID string, nonce uint64, gasPrice *big.Int, gasTipCap *big.Int) (*TxInfo, error)
	SendCosmosTx(t *testing.T, nodeID string, accID string, nonce uint64, gasPrice *big.Int, gasTipCap *big.Int) (*TxInfo, error)

	// Query
	BaseFee() *big.Int
	BaseFeeX2() *big.Int
	WaitForCommit(nodeID string, txHash string, txType string, timeout time.Duration) error
	TxPoolContent(nodeID string, txType string) (pendingTxs, queuedTxs []string, err error)

	// Config
	DefaultTestOption() []TestOption
	GetNodeID(idx int) string

	// Expectation of mempool state
	GetExpPendingTxs() []*TxInfo
	SetExpPendingTxs(txs ...*TxInfo)
	GetExpQueuedTxs() []*TxInfo
	SetExpQueuedTxs(txs ...*TxInfo)
	GetExpDiscardedTxs() []*TxInfo
	PromoteExpTxs(count int)
}
