package suite

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"slices"
	"sync"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/systemtests"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/tests/systemtests/clients"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// SystemTestSuite implements the TestSuite interface and
// provides methods for managing test lifecycle,
// sending transactions, querying state,
// and managing expected mempool state.
type SystemTestSuite struct {
	*systemtests.SystemUnderTest
	options *TestOptions

	// Clients
	EthClient    *clients.EthClient
	CosmosClient *clients.CosmosClient

	// Accounts shared across clients
	accounts     []*TestAccount
	accountsByID map[string]*TestAccount
	accountsMu   sync.Mutex
	accountCond  *sync.Cond

	// Chain management
	chainMu         sync.Mutex
	currentNodeArgs []string

	// Most recently retrieved base fee
	baseFee *big.Int
}

func NewSystemTestSuite(t *testing.T) *SystemTestSuite {
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
			ID:            id,
			Address:       ethAcc.Address,
			AccAddress:    cosmosAcc.AccAddress,
			AccNumber:     cosmosAcc.AccountNumber,
			ECDSAPrivKey:  ethAcc.PrivKey,
			PrivKey:       cosmosAcc.PrivKey,
			Eth:           ethAcc,
			Cosmos:        cosmosAcc,
			perAccountMux: &sync.Mutex{},
		}
		accounts[i] = acc
		accountsByID[id] = acc
	}

	suite := &SystemTestSuite{
		SystemUnderTest: systemtests.Sut,
		EthClient:       ethClient,
		CosmosClient:    cosmosClient,
		accounts:        accounts,
		accountsByID:    accountsByID,
	}
	suite.accountCond = sync.NewCond(&suite.accountsMu)

	return suite
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

	inUse         bool
	perAccountMux *sync.Mutex
}

// SetupTest initializes the test suite by resetting and starting the chain, then awaiting 2 blocks
func (s *SystemTestSuite) SetupTest(t *testing.T, nodeStartArgs ...string) {
	t.Helper()

	if len(nodeStartArgs) == 0 {
		nodeStartArgs = DefaultNodeArgs()
	}

	s.chainMu.Lock()
	defer s.chainMu.Unlock()

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
	s.ensureAdditionalAccountsFunded(t)
}

// LockChain acquires exclusive control over the underlying chain lifecycle.
func (s *SystemTestSuite) LockChain() {
	s.chainMu.Lock()
}

// UnlockChain releases the chain lifecycle lock.
func (s *SystemTestSuite) UnlockChain() {
	s.chainMu.Unlock()
}

func (s *SystemTestSuite) ensureAdditionalAccountsFunded(t *testing.T) {
	const (
		defaultFundedAccounts = 4
		fundingNodeID         = "node0"
	)

	if len(s.accounts) <= defaultFundedAccounts {
		return
	}

	funder := s.Account("acc0")
	require.NotNil(t, funder, "funding account acc0 missing")

	accountInfo := s.mustGetCosmosAccount(t, funder)
	funder.AccNumber = accountInfo.GetAccountNumber()
	funder.Cosmos.AccountNumber = funder.AccNumber
	nextSequence := accountInfo.GetSequence()

	baseFee, err := s.GetLatestBaseFee(fundingNodeID)
	require.NoError(t, err)
	gasPrice := s.GetTxGasPrice(baseFee)
	s.baseFee = baseFee

	for idx := defaultFundedAccounts; idx < len(s.accounts); idx++ {
		target := s.accounts[idx]
		if target.ID == funder.ID {
			continue
		}

		resp, err := s.CosmosClient.BankSend(
			fundingNodeID,
			funder.Cosmos,
			funder.AccAddress,
			target.AccAddress,
			sdkmath.NewInt(1_000_000_000_000),
			nextSequence,
			gasPrice,
		)
		require.NoError(t, err, "failed to fund %s", target.ID)

		err = s.WaitForCommit(fundingNodeID, resp.TxHash, TxTypeCosmos, 30*time.Second)
		require.NoError(t, err, "failed waiting funding tx for %s", target.ID)

		nextSequence++
	}

	s.refreshAccountMetadata(t)
}

func (s *SystemTestSuite) mustGetCosmosAccount(t *testing.T, account *TestAccount) client.Account {
	ctx := s.CosmosClient.ClientCtx.WithClient(s.CosmosClient.RpcClients["node0"])
	s.CosmosClient.ClientCtx = ctx

	acc, err := s.CosmosClient.ClientCtx.AccountRetriever.GetAccount(ctx, account.AccAddress)
	require.NoError(t, err, "failed to fetch cosmos account info for %s", account.ID)
	require.NotNil(t, acc, "cosmos account info missing for %s", account.ID)

	return acc
}

func (s *SystemTestSuite) refreshAccountMetadata(t *testing.T) {
	ctx := s.CosmosClient.ClientCtx.WithClient(s.CosmosClient.RpcClients["node0"])
	s.CosmosClient.ClientCtx = ctx

	for _, account := range s.accounts {
		accInfo, err := s.CosmosClient.ClientCtx.AccountRetriever.GetAccount(ctx, account.AccAddress)
		require.NoError(t, err, "failed to refresh account %s metadata", account.ID)
		require.NotNil(t, accInfo, "account info missing for %s", account.ID)

		account.AccNumber = accInfo.GetAccountNumber()
		account.Cosmos.AccountNumber = account.AccNumber
	}
}

// Lock acquires the mutex guarding this account for exclusive usage.
func (a *TestAccount) Lock() {
	if a.perAccountMux == nil {
		a.perAccountMux = &sync.Mutex{}
	}
	a.perAccountMux.Lock()
}

// Unlock releases the mutex guarding this account.
func (a *TestAccount) Unlock() {
	if a.perAccountMux == nil {
		return
	}
	a.perAccountMux.Unlock()
}
