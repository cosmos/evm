package mempool

import (
	"math/big"
	"testing"
	"time"

	"github.com/cosmos/evm/tests/systemtests/suite"
)

// RunContext carries per-test expected mempool state to avoid shared globals.
type RunContext struct {
	ExpPending []*suite.TxInfo
	ExpQueued  []*suite.TxInfo
}

func NewRunContext() *RunContext {
	return &RunContext{}
}

func (c *RunContext) Reset() {
	c.ExpPending = nil
	c.ExpQueued = nil
}

func (c *RunContext) SetExpPendingTxs(txs ...*suite.TxInfo) {
	c.ExpPending = append(c.ExpPending[:0], txs...)
}

func (c *RunContext) SetExpQueuedTxs(txs ...*suite.TxInfo) {
	c.ExpQueued = append(c.ExpQueued[:0], txs...)
}

func (c *RunContext) PromoteExpTxs(count int) {
	if count <= 0 || len(c.ExpQueued) == 0 {
		return
	}

	if count > len(c.ExpQueued) {
		count = len(c.ExpQueued)
	}

	promoted := c.ExpQueued[:count]
	c.ExpPending = append(c.ExpPending, promoted...)
	c.ExpQueued = c.ExpQueued[count:]
}

type TestSuite interface {
	SetupTest(t *testing.T, nodeStartArgs ...string)

	// Test Lifecycle
	BeforeEachCase(t *testing.T, ctx *RunContext)
	AfterEachCase(t *testing.T, ctx *RunContext)
	AfterEachAction(t *testing.T, ctx *RunContext)

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
	TxPoolContent(nodeID string, txType string, timeout time.Duration) (pendingTxs, queuedTxs []string, err error)

	// Config
	GetOptions() *suite.TestOptions
	SetOptions(options *suite.TestOptions)
	Nodes() []string
	Node(idx int) string
	Acc(idx int) *suite.TestAccount
	AccID(idx int) string
	AcquireAcc() *suite.TestAccount
	ReleaseAcc(acc *suite.TestAccount)

	// Test Utils
	AwaitNBlocks(t *testing.T, n int64, duration ...time.Duration)
	GetTxGasPrice(baseFee *big.Int) *big.Int
}
