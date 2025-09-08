package suite

import (
	"fmt"
	"math/big"
	"slices"
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
	expPendingTxs   []*TxInfo
	expQueuedTxs    []*TxInfo
	expDiscardedTxs []*TxInfo
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

func (s *SystemTestSuite) JustAfterEach(t *testing.T) {
	time.Sleep(1 * time.Second)

	for _, txInfo := range s.GetExpPendingTxs() {
		if txInfo.TxType == TxTypeEVM {
			evmPendingTxHashes, _, err := s.TxPoolContent(txInfo.DstNodeID, TxTypeEVM)
			require.NoError(t, err)

			ok := slices.Contains(evmPendingTxHashes, txInfo.TxHash)
			require.True(t, ok, fmt.Sprintf("tx %s is not contained in pending txs in %s mempool", txInfo.TxHash, txInfo.TxType))
		}
	}

	for _, txInfo := range s.GetExpQueuedTxs() {
		if txInfo.TxType == TxTypeEVM {
			_, evmQueuedTxHashes, err := s.TxPoolContent(txInfo.DstNodeID, TxTypeEVM)
			require.NoError(t, err)

			ok := slices.Contains(evmQueuedTxHashes, txInfo.TxHash)
			require.True(t, ok, fmt.Sprintf("tx %s is not contained in queued txs in %s mempool", txInfo.TxHash, txInfo.TxType))
		}

	}
}

func (s *SystemTestSuite) AfterEach(t *testing.T) {
	for _, txInfo := range s.GetExpPendingTxs() {
		err := s.WaitForCommit(txInfo.DstNodeID, txInfo.TxHash, txInfo.TxType, time.Second*10)
		require.NoError(t, err)
	}

	for _, txInfo := range s.GetExpDiscardedTxs() {
		err := s.WaitForCommit(txInfo.DstNodeID, txInfo.TxHash, txInfo.TxType, time.Second*10)
		require.Error(t, err)
	}
}

func (s *SystemTestSuite) BaseFee() *big.Int {
	return s.baseFee
}

func (s *SystemTestSuite) BaseFeeX2() *big.Int {
	return new(big.Int).Mul(s.baseFee, big.NewInt(2))
}

func (s *SystemTestSuite) GetExpPendingTxs() []*TxInfo {
	return s.expPendingTxs
}

func (s *SystemTestSuite) SetExpPendingTxs(txs ...*TxInfo) {
	s.expPendingTxs = txs
}

func (s *SystemTestSuite) GetExpQueuedTxs() []*TxInfo {
	return s.expQueuedTxs
}

func (s *SystemTestSuite) SetExpQueuedTxs(txs ...*TxInfo) {
	queuedTxs := make([]*TxInfo, 0)
	for _, txInfo := range txs {
		if txInfo.TxType == TxTypeCosmos {
			continue
		}
		queuedTxs = append(queuedTxs, txInfo)
	}
	s.expQueuedTxs = queuedTxs
}

func (s *SystemTestSuite) GetExpDiscardedTxs() []*TxInfo {
	return s.expDiscardedTxs
}

func (s *SystemTestSuite) SetExpDiscardedTxs(txs ...*TxInfo) {
	s.expDiscardedTxs = txs
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
