package suite

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/tests/systemtests/clients"

	"cosmossdk.io/systemtests"

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
	chainMu         sync.Mutex
	currentNodeArgs []string

	// Most recently retrieved base fee
	baseFee *big.Int
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
func RunWithSharedSuite(t *testing.T, fn func(*testing.T, *BaseTestSuite)) {
	t.Helper()
	fn(t, GetSharedSuite(t))
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

// SetupTest initializes the test suite by resetting and starting the chain, then awaiting 2 blocks
func (s *BaseTestSuite) SetupTest(t *testing.T, nodeStartArgs ...string) {
	t.Helper()

	if len(nodeStartArgs) == 0 {
		nodeStartArgs = DefaultNodeArgs()
	}

	s.LockChain()
	defer s.UnlockChain()

	if !s.ChainStarted {
		s.currentNodeArgs = nil
	}

	if s.ChainStarted && slices.Equal(nodeStartArgs, s.currentNodeArgs) {
		// Chain already running with desired configuration; nothing to do.
		return
	}

	if s.ChainStarted {
		s.ResetChain(t)
	}

	s.StartChain(t, nodeStartArgs...)
	s.currentNodeArgs = append([]string(nil), nodeStartArgs...)
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
