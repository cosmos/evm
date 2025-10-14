package mempool

import "github.com/cosmos/evm/tests/systemtests/suite"

// TestContext carries per-test expected mempool state to avoid shared globals.
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
