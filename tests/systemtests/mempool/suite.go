package mempool

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/tests/systemtests/suite"
)

const txPoolContentTimeout = 120 * time.Second

// Suite implements the mempool TestSuite interface over a shared SystemTestSuite.
type Suite struct {
	base *suite.SystemTestSuite
}

func NewSuite(base *suite.SystemTestSuite) *Suite {
	return &Suite{base: base}
}

func (s *Suite) SetupTest(t *testing.T, nodeStartArgs ...string) {
	s.base.SetupTest(t, nodeStartArgs...)
}

func (s *Suite) BeforeEachCase(t *testing.T, ctx *RunContext) {
	ctx.Reset()
	s.base.BeforeEachCase(t)
}

func (s *Suite) AfterEachAction(t *testing.T, ctx *RunContext) {
	require.NoError(t, s.base.CheckTxsPendingAsync(ctx.ExpPending))
	require.NoError(t, s.base.CheckTxsQueuedAsync(ctx.ExpQueued))

	s.base.AwaitNBlocks(t, 1)

	currentBaseFee, err := s.base.GetLatestBaseFee("node0")
	require.NoError(t, err)
	s.base.SetBaseFee(currentBaseFee)
}

func (s *Suite) AfterEachCase(t *testing.T, ctx *RunContext) {
	for _, txInfo := range ctx.ExpPending {
		err := s.base.WaitForCommit(txInfo.DstNodeID, txInfo.TxHash, txInfo.TxType, 60*time.Second)
		require.NoError(t, err)
	}

	for _, nodeID := range s.base.Nodes() {
		pending, _, err := s.base.TxPoolContent(nodeID, suite.TxTypeEVM, txPoolContentTimeout)
		require.NoError(t, err)
		require.Len(t, pending, 0, "pending txs are not cleared in mempool for %s", nodeID)
	}

	for _, nodeID := range s.base.Nodes() {
		pending, _, err := s.base.TxPoolContent(nodeID, suite.TxTypeCosmos, txPoolContentTimeout)
		require.NoError(t, err)
		require.Len(t, pending, 0, "pending cosmos txs are not cleared in mempool for %s", nodeID)
	}

	s.base.AwaitNBlocks(t, 1)
}

// Delegated helpers ---------------------------------------------------------

func (s *Suite) SendTx(t *testing.T, nodeID string, accID string, nonceIdx uint64, gasPrice *big.Int, gasTipCap *big.Int) (*suite.TxInfo, error) {
	return s.base.SendTx(t, nodeID, accID, nonceIdx, gasPrice, gasTipCap)
}

func (s *Suite) SendEthTx(t *testing.T, nodeID string, accID string, nonceIdx uint64, gasPrice *big.Int, gasTipCap *big.Int) (*suite.TxInfo, error) {
	return s.base.SendEthTx(t, nodeID, accID, nonceIdx, gasPrice, gasTipCap)
}

func (s *Suite) SendEthLegacyTx(t *testing.T, nodeID string, accID string, nonceIdx uint64, gasPrice *big.Int) (*suite.TxInfo, error) {
	return s.base.SendEthLegacyTx(t, nodeID, accID, nonceIdx, gasPrice)
}

func (s *Suite) SendEthDynamicFeeTx(t *testing.T, nodeID string, accID string, nonceIdx uint64, gasPrice *big.Int, gasTipCap *big.Int) (*suite.TxInfo, error) {
	return s.base.SendEthDynamicFeeTx(t, nodeID, accID, nonceIdx, gasPrice, gasTipCap)
}

func (s *Suite) SendCosmosTx(t *testing.T, nodeID string, accID string, nonceIdx uint64, gasPrice *big.Int, gasTipCap *big.Int) (*suite.TxInfo, error) {
	return s.base.SendCosmosTx(t, nodeID, accID, nonceIdx, gasPrice, gasTipCap)
}

func (s *Suite) BaseFee() *big.Int {
	return s.base.BaseFee()
}

func (s *Suite) BaseFeeX2() *big.Int {
	return s.base.BaseFeeX2()
}

func (s *Suite) WaitForCommit(nodeID string, txHash string, txType string, timeout time.Duration) error {
	return s.base.WaitForCommit(nodeID, txHash, txType, timeout)
}

func (s *Suite) TxPoolContent(nodeID string, txType string, timeout time.Duration) (pendingTxs, queuedTxs []string, err error) {
	return s.base.TxPoolContent(nodeID, txType, timeout)
}

func (s *Suite) GetOptions() *suite.TestOptions {
	return s.base.GetOptions()
}

func (s *Suite) SetOptions(options *suite.TestOptions) {
	s.base.SetOptions(options)
}

func (s *Suite) Nodes() []string {
	return s.base.Nodes()
}

func (s *Suite) Node(idx int) string {
	return s.base.Node(idx)
}

func (s *Suite) Acc(idx int) *suite.TestAccount {
	return s.base.Acc(idx)
}

func (s *Suite) AccID(idx int) string {
	return s.base.AccID(idx)
}

func (s *Suite) AcquireAcc() *suite.TestAccount {
	return s.base.AcquireAcc()
}

func (s *Suite) ReleaseAcc(acc *suite.TestAccount) {
	s.base.ReleaseAcc(acc)
}

func (s *Suite) AwaitNBlocks(t *testing.T, n int64, duration ...time.Duration) {
	s.base.AwaitNBlocks(t, n, duration...)
}

func (s *Suite) GetTxGasPrice(baseFee *big.Int) *big.Int {
	return s.base.GetTxGasPrice(baseFee)
}
