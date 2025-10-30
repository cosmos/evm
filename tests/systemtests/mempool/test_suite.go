//go:build system_test

package mempool

import (
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
	for _, txInfo := range ctx.ExpPending {
		err := s.WaitForCommit(txInfo.DstNodeID, txInfo.TxHash, txInfo.TxType, txPoolContentTimeout)
		require.NoError(t, err)
	}
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
