package mempool

import (
	"math/big"
	"testing"
	"time"

	"github.com/cosmos/evm/tests/systemtests/suite"
)

type TestSuite interface {
	// Test Lifecycle
	BeforeEachCase(t *testing.T)
	AfterEachCase(t *testing.T)
	AfterEachAction(t *testing.T)

	// Tx
	SendTx(t *testing.T, nodeID string, accID string, nonceIdx uint64, gasPrice *big.Int, gasTipCap *big.Int) (*suite.TxInfo, error)
	SendEthTx(t *testing.T, nodeID string, accID string, nonceIdx uint64, gasPrice *big.Int, gasTipCap *big.Int) (*suite.TxInfo, error)
	SendEthLegacyTx(t *testing.T, nodeID string, accID string, nonceIdx uint64, gasPrice *big.Int) (*suite.TxInfo, error)
	SendEthDynamicFeeTx(t *testing.T, nodeID string, accID string, nonceIdx uint64, gasPrice *big.Int, gasTipCap *big.Int) (*suite.TxInfo, error)
	SendCosmosTx(t *testing.T, nodeID string, accID string, nonceIdx uint64, gasPrice *big.Int, gasTipCap *big.Int) (*suite.TxInfo, error)

	// Query
	BaseFee() *big.Int
	BaseFeeX2() *big.Int
	WaitForCommit(nodeID string, txHash string, txType string, timeout time.Duration) error
	TxPoolContent(nodeID string, txType string) (pendingTxs, queuedTxs []string, err error)

	// Config
	GetOptions() *suite.TestOptions
	Nodes() []string
	Node(idx int) string
	Acc(idx int) string

	// Expectation of mempool state
	GetExpPendingTxs() []*suite.TxInfo
	SetExpPendingTxs(txs ...*suite.TxInfo)
	GetExpQueuedTxs() []*suite.TxInfo
	SetExpQueuedTxs(txs ...*suite.TxInfo)
	PromoteExpTxs(count int)
}
