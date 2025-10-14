//go:build system_test

package eip712

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/systemtests"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/tests/systemtests/clients"
	"github.com/stretchr/testify/require"
)

// SystemTestSuite implements the TestSuite interface for EIP-712 system tests.
type SystemTestSuite struct {
	*systemtests.SystemUnderTest
	CosmosClient *clients.CosmosClient
	EthClient    *clients.EthClient
}

// NewSystemTestSuite creates a new SystemTestSuite instance.
func NewSystemTestSuite(t *testing.T) *SystemTestSuite {
	cosmosClient, err := clients.NewCosmosClient()
	require.NoError(t, err)

	ethClient, err := clients.NewEthClient()
	require.NoError(t, err)

	return &SystemTestSuite{
		SystemUnderTest: systemtests.Sut,
		CosmosClient:    cosmosClient,
		EthClient:       ethClient,
	}
}

// SetupTest initializes the test suite by resetting and starting the chain.
func (s *SystemTestSuite) SetupTest(t *testing.T, nodeStartArgs ...string) {
	if len(nodeStartArgs) == 0 {
		nodeStartArgs = DefaultNodeArgs()
	}

	s.ResetChain(t)
	s.StartChain(t, nodeStartArgs...)
	s.AwaitNBlocks(t, 2)
}

// BeforeEachCase runs before each test case.
func (s *SystemTestSuite) BeforeEachCase(t *testing.T) {
	// Setup before each test case if needed
}

// AfterEachCase runs after each test case.
func (s *SystemTestSuite) AfterEachCase(t *testing.T) {
	// Wait for a block to ensure transactions are processed
	s.AwaitNBlocks(t, 1)
}

// SendBankSendWithEIP712 sends a bank send transaction using EIP-712 signing.
func (s *SystemTestSuite) SendBankSendWithEIP712(
	t *testing.T,
	nodeID string,
	accID string,
	to sdk.AccAddress,
	amount *big.Int,
	nonce uint64,
	gasPrice *big.Int,
) (string, error) {
	from := s.CosmosClient.Accs[accID].AccAddress

	resp, err := BankSendWithEIP712(
		s.CosmosClient,
		nodeID,
		accID,
		from,
		to,
		sdkmath.NewIntFromBigInt(amount),
		nonce,
		gasPrice,
	)
	if err != nil {
		return "", fmt.Errorf("failed to send bank send with EIP-712: %w", err)
	}

	return resp.TxHash, nil
}

// GetBalance retrieves the balance of a given address for a specific denomination.
func (s *SystemTestSuite) GetBalance(
	t *testing.T,
	nodeID string,
	address sdk.AccAddress,
	denom string,
) (*big.Int, error) {
	balance, err := s.CosmosClient.GetBalance(nodeID, address, denom)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %v", err)
	}

	return balance, nil
}

// WaitForCommit waits for a transaction to be committed.
func (s *SystemTestSuite) WaitForCommit(nodeID string, txHash string, timeout ...int) error {
	timeoutDuration := 15 * time.Second
	if len(timeout) > 0 && timeout[0] > 0 {
		timeoutDuration = time.Duration(timeout[0]) * time.Second
	}

	result, err := s.CosmosClient.WaitForCommit(nodeID, txHash, timeoutDuration)
	if err != nil {
		return fmt.Errorf("failed to wait for commit: %v", err)
	}

	if result.TxResult.Code != 0 {
		return fmt.Errorf("transaction failed with code %d: %s", result.TxResult.Code, result.TxResult.Log)
	}

	return nil
}

// Node returns the node ID for the given index.
func (s *SystemTestSuite) Node(idx int) string {
	return fmt.Sprintf("node%d", idx)
}

// Acc returns the account ID for the given index.
func (s *SystemTestSuite) Acc(idx int) string {
	return fmt.Sprintf("acc%d", idx)
}

// AwaitNBlocks waits for N blocks to be produced.
func (s *SystemTestSuite) AwaitNBlocks(t *testing.T, n int64) {
	s.SystemUnderTest.AwaitNBlocks(t, n)
}

// DefaultNodeArgs returns the default node startup arguments.
func DefaultNodeArgs() []string {
	chainID := "--chain-id=local-4221"
	evmChainID := "--evm.evm-chain-id=4221"
	apiEnable := "--api.enable=true"
	jsonrpcApi := "--json-rpc.api=eth,txpool,personal,net,debug,web3"
	jsonrpcAllowUnprotectedTxs := "--json-rpc.allow-unprotected-txs=true"
	maxTxs := "--mempool.max-txs=0"

	return []string{jsonrpcApi, chainID, evmChainID, apiEnable, jsonrpcAllowUnprotectedTxs, maxTxs}
}
