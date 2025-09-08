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

	options *TestOptions

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
	s.AwaitNBlocks(t, 2)
}

func (s *SystemTestSuite) BeforeEach(t *testing.T) {
	// Reset expected pending/queued transactions
	s.SetExpPendingTxs()
	s.SetExpQueuedTxs()

	// Get current base fee
	currentBaseFee, err := s.GetLatestBaseFee("node0")
	require.NoError(t, err)

	s.baseFee = currentBaseFee
}

func (s *SystemTestSuite) JustAfterEach(t *testing.T) {
	for _, txInfo := range s.GetExpPendingTxs() {
		err := s.CheckPendingOrCommitted(txInfo.DstNodeID, txInfo.TxHash, txInfo.TxType, time.Second*15)
		require.NoError(t, err, "tx is not pending or committed")
	}

	for _, txInfo := range s.GetExpQueuedTxs() {
		_, evmQueuedTxHashes, err := s.TxPoolContent(txInfo.DstNodeID, txInfo.TxType)
		require.NoError(t, err)

		ok := slices.Contains(evmQueuedTxHashes, txInfo.TxHash)
		require.True(t, ok, fmt.Sprintf("tx %s is not contained in queued txs in %s mempool", txInfo.TxHash, txInfo.TxType))
	}

	// Wait for block commit
	s.AwaitNBlocks(t, 1)

	// Get current base fee
	currentBaseFee, err := s.GetLatestBaseFee("node0")
	require.NoError(t, err)

	s.baseFee = currentBaseFee
}

func (s *SystemTestSuite) AfterEach(t *testing.T) {
	for _, txInfo := range s.GetExpPendingTxs() {
		err := s.WaitForCommit(txInfo.DstNodeID, txInfo.TxHash, txInfo.TxType, time.Second*15)
		require.NoError(t, err)
	}

	for _, txInfo := range s.GetExpDiscardedTxs() {
		err := s.WaitForCommit(txInfo.DstNodeID, txInfo.TxHash, txInfo.TxType, time.Second*15)
		require.Error(t, err)
	}

	// Wait for block commit
	s.AwaitNBlocks(t, 1)
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

func (s *SystemTestSuite) Node(idx int) string {
	return fmt.Sprintf("node%d", idx)
}

func (s *SystemTestSuite) Acc(idx int) string {
	return fmt.Sprintf("acc%d", idx)
}

func (s *SystemTestSuite) GetOptions() *TestOptions {
	return s.options
}

func (s *SystemTestSuite) SetOptions(options *TestOptions) {
	s.options = options
}
