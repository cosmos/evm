package suite

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"cosmossdk.io/systemtests"
	"github.com/cosmos/evm/tests/systemtests/clients"
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

	// Most recently retrieved base fee
	baseFee *big.Int

	// Expected transaction hashes
	expPendingTxs []*TxInfo
	expQueuedTxs  []*TxInfo
}

func NewSystemTestSuite(t *testing.T) *SystemTestSuite {
	ethClient, err := clients.NewEthClient()
	require.NoError(t, err)

	cosmosClient, err := clients.NewCosmosClient()
	require.NoError(t, err)

	return &SystemTestSuite{
		SystemUnderTest: systemtests.Sut,
		EthClient:       ethClient,
		CosmosClient:    cosmosClient,
	}
}

// SetupTest initializes the test suite by resetting and starting the chain, then awaiting 2 blocks
func (s *SystemTestSuite) SetupTest(t *testing.T) {
	s.ResetChain(t)
	s.StartChain(t, DefaultNodeArgs()...)
	s.AwaitNBlocks(t, 2)
}

// BeforeEach resets the expected mempool state and retrieves the current base fee before each test case
func (s *SystemTestSuite) BeforeEach(t *testing.T) {
	// Reset expected pending/queued transactions
	s.SetExpPendingTxs()
	s.SetExpQueuedTxs()

	// Get current base fee
	currentBaseFee, err := s.GetLatestBaseFee("node0")
	require.NoError(t, err)

	s.baseFee = currentBaseFee
}

// JustAfterEach checks the expected mempool state right after each test case
func (s *SystemTestSuite) JustAfterEach(t *testing.T) {
	for _, txInfo := range s.GetExpPendingTxs() {
		err := s.CheckPendingOrCommitted(txInfo.DstNodeID, txInfo.TxHash, txInfo.TxType, time.Second*15)
		require.NoError(t, err, "tx is not pending or committed")
	}

	// Should check queued txs does not exist in other node's mempool
	// This query should be done using go routine because it may take time.
	for _, txInfo := range s.GetExpQueuedTxs() {
		if txInfo.TxType != TxTypeEVM {
			panic("queued txs should be only EVM txs")
		}

		for _, nodeID := range s.Nodes() {
			if nodeID == txInfo.DstNodeID {
				err := s.EthClient.CheckQueued(txInfo.DstNodeID, txInfo.TxHash, time.Second*15)
				require.NoError(t, err, fmt.Sprintf("tx %s is not contained in queued txs in %s mempool", txInfo.TxHash, txInfo.TxType))
			} else {
				err := s.EthClient.CheckNotReceived(nodeID, txInfo.TxHash, time.Second*15)
				require.NoError(t, err)
			}
		}
	}

	// Wait for block commit
	s.AwaitNBlocks(t, 1)

	// Get current base fee
	currentBaseFee, err := s.GetLatestBaseFee("node0")
	require.NoError(t, err)

	s.baseFee = currentBaseFee
}

// AfterEach waits for all expected pending transactions to be committed
func (s *SystemTestSuite) AfterEach(t *testing.T) {
	for _, txInfo := range s.GetExpPendingTxs() {
		err := s.WaitForCommit(txInfo.DstNodeID, txInfo.TxHash, txInfo.TxType, time.Second*15)
		require.NoError(t, err)
	}

	// Wait for block commit
	s.AwaitNBlocks(t, 1)
}
