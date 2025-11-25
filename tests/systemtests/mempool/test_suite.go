package mempool

import (
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/tests/systemtests/suite"
)

const txPoolContentTimeout = 120 * time.Second

// Suite wraps the shared BaseTestSuite with mempool-specific helpers.
type TestSuite struct {
	*suite.BaseTestSuite
}

func NewTestSuite(base *suite.BaseTestSuite) *TestSuite {
	return &TestSuite{BaseTestSuite: base}
}

func (s *TestSuite) SetupTest(t *testing.T, nodeStartArgs ...string) {
	s.BaseTestSuite.SetupTest(t, nodeStartArgs...)
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
	if err != nil {
		// If we fail to get the latest base fee, we just keep the previous one
		currentBaseFee = s.BaseFee()
	}
	s.SetBaseFee(currentBaseFee)
}

func (s *TestSuite) AfterEachCase(t *testing.T, ctx *TestContext) {
	t.Logf("=== AfterEachCase: Starting verification ===")
	t.Logf("AfterEachCase: Waiting for %d transactions to commit", len(ctx.ExpPending))

	// Log all expected transactions first
	for i, txInfo := range ctx.ExpPending {
		t.Logf("Expected tx %d/%d: hash=%s, node=%s, type=%s",
			i+1, len(ctx.ExpPending), txInfo.TxHash, txInfo.DstNodeID, txInfo.TxType)
	}

	// Now wait for each transaction
	for i, txInfo := range ctx.ExpPending {
		t.Logf("Waiting for tx %d/%d to commit: hash=%s, node=%s, type=%s",
			i+1, len(ctx.ExpPending), txInfo.TxHash, txInfo.DstNodeID, txInfo.TxType)
		err := s.WaitForCommit(txInfo.DstNodeID, txInfo.TxHash, txInfo.TxType, txPoolContentTimeout)
		if err != nil {
			t.Logf("ERROR: Failed to wait for tx %d/%d (hash=%s, node=%s): %v",
				i+1, len(ctx.ExpPending), txInfo.TxHash, txInfo.DstNodeID, err)

			// On error, dump mempool state from all nodes for debugging
			t.Logf("Dumping mempool state from all nodes for debugging:")
			for _, nodeID := range s.Nodes() {
				pending, queued, mempoolErr := s.TxPoolContent(nodeID, txInfo.TxType, 5*time.Second)
				if mempoolErr != nil {
					t.Logf("  %s: Failed to get mempool state: %v", nodeID, mempoolErr)
				} else {
					t.Logf("  %s: pending=%d, queued=%d", nodeID, len(pending), len(queued))
					if len(pending) > 0 {
						t.Logf("    Pending: %v", pending)
					}
					if len(queued) > 0 {
						t.Logf("    Queued: %v", queued)
					}
				}
			}

			// Dump node logs for debugging
			t.Logf("Dumping node logs for debugging (last 100 lines from each node):")
			s.DumpAllNodeLogs(t, 100)
		} else {
			t.Logf("SUCCESS: Tx %d/%d committed (hash=%s)", i+1, len(ctx.ExpPending), txInfo.TxHash)
		}
		require.NoError(t, err)
	}
	t.Logf("=== AfterEachCase: All %d transactions committed successfully ===", len(ctx.ExpPending))
}

type TestContext struct {
	ExpPending []*suite.TxInfo
	ExpQueued  []*suite.TxInfo
}

func NewTestContext() *TestContext {
	return &TestContext{}
}

func (c *TestContext) Reset() {
	c.ExpPending = nil
	c.ExpQueued = nil
}

func (c *TestContext) SetExpPendingTxs(txs ...*suite.TxInfo) {
	c.ExpPending = append(c.ExpPending[:0], txs...)
}

func (c *TestContext) SetExpQueuedTxs(txs ...*suite.TxInfo) {
	c.ExpQueued = append(c.ExpQueued[:0], txs...)
}

func (c *TestContext) PromoteExpTxs(count int) {
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

// DumpNodeLogs dumps the last N lines of node logs for debugging
func (s *TestSuite) DumpNodeLogs(t *testing.T, nodeID string, tailLines int) {
	t.Helper()
	logPath := fmt.Sprintf("testnet/%s.out", nodeID)

	t.Logf("=== Last %d lines of %s logs ===", tailLines, nodeID)

	// Try to read the last N lines of the log file
	content, err := exec.Command("tail", "-n", fmt.Sprintf("%d", tailLines), logPath).Output()
	if err != nil {
		t.Logf("Failed to read logs from %s: %v", logPath, err)
		return
	}

	t.Logf("%s", string(content))
	t.Logf("=== End of %s logs ===", nodeID)
}

// DumpAllNodeLogs dumps logs from all nodes
func (s *TestSuite) DumpAllNodeLogs(t *testing.T, tailLines int) {
	t.Helper()
	for _, nodeID := range s.Nodes() {
		s.DumpNodeLogs(t, nodeID, tailLines)
	}
}
