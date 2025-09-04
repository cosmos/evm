package suite

import (
	"math/big"
	"testing"
	"time"

	"cosmossdk.io/systemtests"
	"github.com/cosmos/evm/tests/systemtests/clients"
	"github.com/stretchr/testify/require"
)

type SystemTestSuite struct {
	*systemtests.SystemUnderTest
	EthClient    *clients.EthClient
	CosmosClient *clients.CosmosClient

	baseFee *big.Int

	TestOption TestOption

	nodeIterator *NodeIterator

	// Transaction Hashes
	expPendingTxs []string
	expQueuedTxs  []string
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

func (s *SystemTestSuite) SetupTest(t *testing.T) {
	s.ResetChain(t)
	s.StartChain(t, DefaultNodeArgs()...)
	s.AwaitNBlocks(t, 10)
}

func (s *SystemTestSuite) BeforeEach(t *testing.T) {
	// Reset expected pending/queued transactions
	s.SetExpPendingTxs()
	s.SetExpQueuedTxs()

	// Reset nodeIterator
	s.nodeIterator = NewNodeIterator(s.TestOption.NodeEntries)

	// Get current base fee
	currentBaseFee, err := s.GetLatestBaseFee("node0")
	require.NoError(t, err)

	s.baseFee = currentBaseFee
}

func (s *SystemTestSuite) AfterEach(t *testing.T) {
	for _, txHash := range s.GetExpPendingTxs() {
		err := s.WaitForCommit("node0", txHash, time.Second*15)
		require.NoError(t, err)
	}
}

func (s *SystemTestSuite) BaseFee() *big.Int {
	return s.baseFee
}

func (s *SystemTestSuite) BaseFeeX2() *big.Int {
	return new(big.Int).Mul(s.baseFee, big.NewInt(2))
}

func (s *SystemTestSuite) OnlyEthTxs() bool {
	return s.TestOption.TxType == TxTypeEVM
}

func (s *SystemTestSuite) GetExpPendingTxs() []string {
	return s.expPendingTxs
}

func (s *SystemTestSuite) GetExpPendingTx(idx int) string {
	return s.expPendingTxs[idx]
}

func (s *SystemTestSuite) SetExpPendingTxs(txs ...string) {
	s.expPendingTxs = txs
}

func (s *SystemTestSuite) GetExpQueuedTxs() []string {
	return s.expQueuedTxs
}

func (s *SystemTestSuite) GetExpQueuedTx(idx int) string {
	return s.expQueuedTxs[idx]
}

func (s *SystemTestSuite) SetExpQueuedTxs(txs ...string) {
	s.expQueuedTxs = txs
}

func (s *SystemTestSuite) PromoteExpTxs(count int) {
	if count <= 0 || len(s.expQueuedTxs) == 0 {
		return
	}

	// Ensure we don't try to promote more than available
	actualCount := count
	if actualCount > len(s.expQueuedTxs) {
		actualCount = len(s.expQueuedTxs)
	}

	// Pop from expQueuedTxs and push to expPendingTxs
	txs := s.expQueuedTxs[:actualCount]
	s.expPendingTxs = append(s.expPendingTxs, txs...)
	s.expQueuedTxs = s.expQueuedTxs[actualCount:]
}

func (s *SystemTestSuite) GetNode() string {
	if s.nodeIterator == nil || s.nodeIterator.IsEmpty() {
		return "node0"
	}

	currentNode := s.nodeIterator.Node()
	s.nodeIterator.Next()

	return currentNode
}
