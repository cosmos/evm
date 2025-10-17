package mempool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/tests/systemtests/suite"
)

const txPoolContentTimeout = 120 * time.Second

// Suite wraps the shared SystemTestSuite with mempool-specific helpers.
type TestSuite struct {
	*suite.SystemTestSuite
}

func NewSuite(base *suite.SystemTestSuite) *TestSuite {
	return &TestSuite{SystemTestSuite: base}
}

func (s *TestSuite) SetupTest(t *testing.T, nodeStartArgs ...string) {
	s.SystemTestSuite.SetupTest(t, nodeStartArgs...)
}

// BeforeEach resets the expected mempool state and retrieves the current base fee before each test case
func (s *TestSuite) BeforeEachCase(t *testing.T, ctx *TestContext) {
	ctx.Reset()

	// Get current base fee
	currentBaseFee, err := s.GetLatestBaseFee("node0")
	require.NoError(t, err)

	s.SetBaseFee(currentBaseFee)
}

func (s *TestSuite) AfterEachAction(t *testing.T, ctx *TestContext) {
	require.NoError(t, s.CheckTxsPendingAsync(ctx.ExpPending))
	require.NoError(t, s.CheckTxsQueuedAsync(ctx.ExpQueued))

	currentBaseFee, err := s.GetLatestBaseFee("node0")
	require.NoError(t, err)
	s.SetBaseFee(currentBaseFee)
}

func (s *TestSuite) AfterEachCase(t *testing.T, ctx *TestContext) {
	for _, txInfo := range ctx.ExpPending {
		err := s.WaitForCommit(txInfo.DstNodeID, txInfo.TxHash, txInfo.TxType, 60*time.Second)
		require.NoError(t, err)
	}

	for _, nodeID := range s.Nodes() {
		pending, _, err := s.TxPoolContent(nodeID, suite.TxTypeEVM, txPoolContentTimeout)
		require.NoError(t, err)
		require.Len(t, pending, 0, "pending txs are not cleared in mempool for %s", nodeID)
	}

	for _, nodeID := range s.Nodes() {
		pending, _, err := s.TxPoolContent(nodeID, suite.TxTypeCosmos, txPoolContentTimeout)
		require.NoError(t, err)
		require.Len(t, pending, 0, "pending cosmos txs are not cleared in mempool for %s", nodeID)
	}
}
