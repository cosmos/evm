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
<<<<<<< HEAD
	"github.com/cosmos/evm/tests/systemtests/clients"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
=======

	sdk "github.com/cosmos/cosmos-sdk/types"
>>>>>>> main
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

<<<<<<< HEAD
	s.ResetChain(t)
	s.ModifyGenesisJSON(t, setupTestDenomMetadata(t))
=======
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

>>>>>>> main
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

// setupTestDenomMetadata returns a function that sets up the required denom metadata for "atest" in genesis
func setupTestDenomMetadata(t *testing.T) func(genesis []byte) []byte {
	return func(genesis []byte) []byte {
		// Set up denom metadata for "atest" as a proper JSON object
		denomMetadata := `{"description":"The native staking token for system tests.","denom_units":[{"denom":"atest","exponent":0,"aliases":["attotest"]},{"denom":"test","exponent":18,"aliases":[]}],"base":"atest","display":"test","name":"Test Token","symbol":"TEST","uri":"","uri_hash":""}`

		// Add denom metadata to bank module as an array with one element using SetRaw
		genesis, err := sjson.SetRawBytes(genesis, "app_state.bank.denom_metadata.0", []byte(denomMetadata))
		require.NoError(t, err)

		// Also add "stake" denom metadata for the genesis transaction
		stakeMetadata := `{"description":"The native staking token for genesis.","denom_units":[{"denom":"stake","exponent":0,"aliases":[]},{"denom":"stake","exponent":0,"aliases":[]}],"base":"stake","display":"stake","name":"Stake Token","symbol":"STAKE","uri":"","uri_hash":""}`
		genesis, err = sjson.SetRawBytes(genesis, "app_state.bank.denom_metadata.1", []byte(stakeMetadata))
		require.NoError(t, err)

		// Set EVM module to use "atest" as the EVM denomination
		genesis, err = sjson.SetBytes(genesis, "app_state.evm.params.evm_denom", "atest")
		require.NoError(t, err)

		// Set staking bond denom to "stake" to match genesis transaction
		genesis, err = sjson.SetBytes(genesis, "app_state.staking.params.bond_denom", "stake")
		require.NoError(t, err)

		// Set mint denom to "atest"
		genesis, err = sjson.SetBytes(genesis, "app_state.mint.params.mint_denom", "atest")
		require.NoError(t, err)

		// Set gov deposit denom to "atest"
		genesis, err = sjson.SetBytes(genesis, "app_state.gov.params.min_deposit.0.denom", "atest")
		require.NoError(t, err)

		genesis, err = sjson.SetBytes(genesis, "app_state.gov.params.expedited_min_deposit.0.denom", "atest")
		require.NoError(t, err)

		return genesis
	}
}
