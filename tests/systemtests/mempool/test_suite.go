//go:build system_test

package mempool

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/creachadair/tomledit"
	"github.com/creachadair/tomledit/parser"
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

// GetCurrentBlockHeight returns the current block height from the specified node
func (s *TestSuite) GetCurrentBlockHeight(t *testing.T, nodeID string) uint64 {
	t.Helper()
	account := s.EthAccount("acc0")
	ctx, cli, _ := s.EthClient.Setup(nodeID, account)
	blockNumber, err := cli.BlockNumber(ctx)
	require.NoError(t, err, "failed to get block number from %s", nodeID)
	return blockNumber
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

// ModifyConsensusTimeout modifies the consensus timeout_commit in the config.toml
// for all nodes and restarts the chain with the new configuration.
func (s *TestSuite) ModifyConsensusTimeout(t *testing.T, timeout string) {
	t.Helper()

	// Stop the chain if running
	if s.ChainStarted {
		s.ResetChain(t)
	}

	// Modify config.toml for each node
	for i := 0; i < s.NodesCount(); i++ {
		nodeDir := s.NodeDir(i)
		configPath := filepath.Join(nodeDir, "config", "config.toml")

		err := editToml(configPath, func(doc *tomledit.Document) {
			setValue(doc, timeout, "consensus", "timeout_commit")
		})
		require.NoError(t, err, "failed to modify config.toml for node %d", i)
	}

	// Restart the chain with modified config
	s.StartChain(t, suite.DefaultNodeArgs()...)
	s.AwaitNBlocks(t, 2)
}

// editToml is a helper to edit TOML files
func editToml(filename string, f func(doc *tomledit.Document)) error {
	tomlFile, err := os.OpenFile(filename, os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer tomlFile.Close()

	doc, err := tomledit.Parse(tomlFile)
	if err != nil {
		return fmt.Errorf("failed to parse toml: %w", err)
	}

	f(doc)

	if _, err := tomlFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}
	if err := tomlFile.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate: %w", err)
	}
	if err := tomledit.Format(tomlFile, doc); err != nil {
		return fmt.Errorf("failed to format: %w", err)
	}

	return nil
}

// setValue sets a value in a TOML document
func setValue(doc *tomledit.Document, newVal string, xpath ...string) {
	e := doc.First(xpath...)
	if e == nil {
		panic(fmt.Sprintf("not found: %v", xpath))
	}
	e.Value = parser.MustValue(fmt.Sprintf("%q", newVal))
}
