package mempool

import (
	"math/big"
	"testing"
	"time"

	"github.com/cosmos/evm/tests/systemtests/suite"
)

type TestSuite interface {
	// Test Lifecycle
	BeforeEach(t *testing.T)
	AfterEach(t *testing.T)
	JustAfterEach(t *testing.T)

	// Tx
	SendTx(nodeID string, accID string, nonce uint64, gasPrice *big.Int, gasTipCap *big.Int) (string, error)
	SendEthTx(nodeID string, accID string, nonce uint64, gasPrice *big.Int, gasTipCap *big.Int) (string, error)
	SendCosmosTx(nodeID string, accID string, nonce uint64, gasPrice *big.Int, gasTipCap *big.Int) (string, error)

	// Query
	BaseFee() *big.Int
	BaseFeeX2() *big.Int
	WaitForCommit(nodeID string, txHash string, timeout time.Duration) error
	TxPoolContent(nodeID string) (pendingTxs, queuedTxs []string, err error)

	// Config
	DefaultTestOption() []suite.TestOption
	OnlyEthTxs() bool
	GetNode() string

	// Expectation of mempool state
	GetExpPendingTxs() []string
	SetExpPendingTxs(txs ...string)
	GetExpQueuedTxs() []string
	SetExpQueuedTxs(txs ...string)
	PromoteExpTxs(count int)
}
