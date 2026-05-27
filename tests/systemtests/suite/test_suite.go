package suite

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/creachadair/tomledit"
	"github.com/creachadair/tomledit/parser"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/tests/systemtests/clients"

	"github.com/cosmos/cosmos-sdk/tools/systemtests"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BaseTestSuite implements the TestSuite interface and
// provides methods for managing test lifecycle,
// sending transactions, querying state,
// and managing expected mempool state.
type BaseTestSuite struct {
	*systemtests.SystemUnderTest
	options *TestOptions

	// Clients
	EthClient    *clients.EthClient
	CosmosClient *clients.CosmosClient

	// Accounts shared across clients
	accounts     []*TestAccount
	accountsByID map[string]*TestAccount

	// Chain management
	chainMu           sync.Mutex
	currentNodeArgs   []string
	currentNodeConfig TestSetupConfig

	// Most recently retrieved base fee
	baseFee *big.Int

	// Extra node start args on top of default
	nodeStartArgs []string
}

func NewBaseTestSuite(t *testing.T) *BaseTestSuite {
	ethClient, ethAccounts, err := clients.NewEthClient()
	require.NoError(t, err)

	cosmosClient, cosmosAccounts, err := clients.NewCosmosClient()
	require.NoError(t, err)

	accountCount := len(ethAccounts)
	require.Equal(t, accountCount, len(cosmosAccounts), "ethereum/cosmos account mismatch")
	accounts := make([]*TestAccount, accountCount)
	accountsByID := make(map[string]*TestAccount, accountCount)
	for i := 0; i < accountCount; i++ {
		id := fmt.Sprintf("acc%d", i)
		ethAcc, ok := ethAccounts[id]
		require.Truef(t, ok, "ethereum account %s not found", id)
		cosmosAcc, ok := cosmosAccounts[id]
		require.Truef(t, ok, "cosmos account %s not found", id)
		acc := &TestAccount{
			ID:           id,
			Address:      ethAcc.Address,
			AccAddress:   cosmosAcc.AccAddress,
			AccNumber:    cosmosAcc.AccountNumber,
			ECDSAPrivKey: ethAcc.PrivKey,
			PrivKey:      cosmosAcc.PrivKey,
			Eth:          ethAcc,
			Cosmos:       cosmosAcc,
		}
		accounts[i] = acc
		accountsByID[id] = acc
	}

	suite := &BaseTestSuite{
		SystemUnderTest: systemtests.Sut,
		EthClient:       ethClient,
		CosmosClient:    cosmosClient,
		accounts:        accounts,
		accountsByID:    accountsByID,
	}
	return suite
}

var (
	sharedSuiteOnce sync.Once
	sharedSuite     *BaseTestSuite
)

func GetSharedSuite(t *testing.T) *BaseTestSuite {
	t.Helper()

	sharedSuiteOnce.Do(func() {
		sharedSuite = NewBaseTestSuite(t)
	})

	return sharedSuite
}

// RunWithSharedSuite retrieves the shared suite instance and executes the provided test function.
func RunWithSharedSuite(t *testing.T, fn func(*testing.T, *BaseTestSuite), nodeStartArgs ...string) {
	t.Helper()
	suite := GetSharedSuite(t)
	suite.SetNodeStartArgs(nodeStartArgs...)
	fn(t, suite)
}

func (suite *BaseTestSuite) SetNodeStartArgs(nodeStartArgs ...string) {
	suite.nodeStartArgs = nodeStartArgs
}

// TestAccount aggregates account metadata usable across both Ethereum and Cosmos flows.
type TestAccount struct {
	ID         string
	Address    common.Address
	AccAddress sdk.AccAddress
	AccNumber  uint64

	ECDSAPrivKey *ecdsa.PrivateKey
	PrivKey      *ethsecp256k1.PrivKey

	Eth    *clients.EthAccount
	Cosmos *clients.CosmosAccount
}

type TestSetupConfig struct {
	timeoutCommit time.Duration
}

func (tc TestSetupConfig) Equals(other TestSetupConfig) bool {
	return tc.timeoutCommit == other.timeoutCommit
}

type TestSetupConfigOption func(*TestSetupConfig)

func WithTimeoutCommit(tc time.Duration) TestSetupConfigOption {
	return func(tsc *TestSetupConfig) {
		tsc.timeoutCommit = tc
	}
}

// SetupTest initializes the test suite by resetting and starting the chain, then awaiting 2 blocks
func (s *BaseTestSuite) SetupTest(t *testing.T, opts ...TestSetupConfigOption) {
	t.Helper()

	var cfg TestSetupConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	if len(s.nodeStartArgs) == 0 {
		s.nodeStartArgs = DefaultNodeArgs()
	}

	s.LockChain()
	defer s.UnlockChain()

	if !s.ChainStarted {
		s.currentNodeArgs = nil
		s.currentNodeConfig = TestSetupConfig{}
	}

	if s.ChainStarted && slices.Equal(s.nodeStartArgs, s.currentNodeArgs) && s.currentNodeConfig == cfg {
		// Chain already running with desired configuration; nothing to do.
		return
	}

	if s.ChainStarted {
		s.ResetChain(t)
	}

	s.ModifyCometMempool(t, "app")

	if cfg.timeoutCommit > time.Duration(0) {
		s.ModifyConsensusTimeout(t, cfg.timeoutCommit.String())
	} else {
		// if not set, default to 2s
		s.ModifyConsensusTimeout(t, time.Duration(2*time.Second).String())
	}

	s.StartChain(t, s.nodeStartArgs...)
	s.currentNodeConfig = cfg
	s.AwaitNBlocks(t, 2)
}

// LockChain acquires exclusive control over the underlying chain lifecycle.
func (s *BaseTestSuite) LockChain() {
	s.chainMu.Lock()
}

// UnlockChain releases the chain lifecycle lock.
func (s *BaseTestSuite) UnlockChain() {
	s.chainMu.Unlock()
}

// ModifyCometMempool modifies the mempool type in the config.toml
func (s *BaseTestSuite) ModifyCometMempool(t *testing.T, mempoolType string) {
	t.Helper()

	// Modify config.toml for each node
	for i := 0; i < s.NodesCount(); i++ {
		nodeDir := s.NodeDir(i)
		configPath := filepath.Join(nodeDir, "config", "config.toml")

		err := editToml(configPath, func(doc *tomledit.Document) {
			setValue(doc, mempoolType, "mempool", "type")
		})
		require.NoError(t, err, "failed to modify config.toml for node %d", i)
	}
}

// ModifyConsensusTimeout modifies the consensus timeout_commit in the config.toml
// for all nodes and restarts the chain with the new configuration.
func (s *BaseTestSuite) ModifyConsensusTimeout(t *testing.T, timeout string) {
	t.Helper()

	// Modify config.toml for each node
	for i := 0; i < s.NodesCount(); i++ {
		nodeDir := s.NodeDir(i)
		configPath := filepath.Join(nodeDir, "config", "config.toml")

		err := editToml(configPath, func(doc *tomledit.Document) {
			setValue(doc, timeout, "consensus", "timeout_commit")
		})
		require.NoError(t, err, "failed to modify config.toml for node %d", i)
	}
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
