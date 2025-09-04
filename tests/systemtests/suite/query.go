package suite

import (
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

func (s *SystemTestSuite) NonceAt(nodeID string, accID string) (uint64, error) {
	ctx, cli, addr := s.EthClient.Setup(nodeID, accID)
	blockNumber, err := s.EthClient.Clients[nodeID].BlockNumber(ctx)
	if err != nil {
		return uint64(0), fmt.Errorf("failed to get block number from %s", nodeID)
	}
	if int64(blockNumber) < 0 {
		return uint64(0), fmt.Errorf("invaid block number %d", blockNumber)
	}
	return cli.NonceAt(ctx, addr, big.NewInt(int64(blockNumber)))
}

func (s *SystemTestSuite) GetLatestBaseFee(nodeID string) (*big.Int, error) {
	ctx, cli, _ := s.EthClient.Setup(nodeID, "acc0")
	blockNumber, err := cli.BlockNumber(ctx)
	if err != nil {
		return nil, err
	}

	block, err := cli.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, err
	}

	return block.BaseFee(), nil
}

func (s *SystemTestSuite) WaitForCommit(
	nodeID string,
	txHash string,
	timeout time.Duration,
) error {
	if s.TestOption.TestType == TxTypeEVM {
		return s.waitForEthCommmit(nodeID, txHash, timeout)
	}
	return s.waitForCosmosCommmit(nodeID, txHash, timeout)
}

func (s *SystemTestSuite) waitForEthCommmit(
	nodeID string,
	txHash string,
	timeout time.Duration,
) error {
	receipt, err := s.EthClient.WaitForCommit(nodeID, txHash, timeout)
	if err != nil {
		return fmt.Errorf("failed to get receipt for tx(%s): %v", txHash, err)
	}

	if receipt.Status != 1 {
		return fmt.Errorf("tx(%s) is committed but failed: %v", txHash, err)
	}

	return nil
}

func (s *SystemTestSuite) waitForCosmosCommmit(
	nodeID string,
	txHash string,
	timeout time.Duration,
) error {
	result, err := s.CosmosClient.WaitForCommit(nodeID, txHash, timeout)
	if err != nil {
		return fmt.Errorf("failed to get receipt for tx(%s): %v", txHash, err)
	}

	if result.TxResult.Code != 0 {
		return fmt.Errorf("tx(%s) is committed but failed: %v", result.Hash.String(), err)
	}

	return nil
}

func (s *SystemTestSuite) TxPoolContent(nodeID string) (pendingTxs, queuedTxs []string, err error) {
	if s.TestOption.TestType == TxTypeEVM {
		return s.ethTxPoolContent(nodeID)
	}
	return s.cosmosTxPoolContent(nodeID)
}

func (s *SystemTestSuite) ethTxPoolContent(nodeID string) (pendingTxHashes, queuedTxHashes []string, err error) {
	pendingTxs, queuedTxs, err := s.EthClient.TxPoolContent(nodeID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get txpool content from eth client")
	}

	return s.extractTxHashesSorted(pendingTxs), s.extractTxHashesSorted(queuedTxs), nil
}

func (s *SystemTestSuite) cosmosTxPoolContent(nodeID string) (pendingTxHashes, queuedTxHashes []string, err error) {
	result, err := s.CosmosClient.UnconfirmedTxs(nodeID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to call unconfired transactions from cosmos cliemt")
	}

	pendingtxHashes := make([]string, 0)
	for _, tx := range result.Txs {
		pendingtxHashes = append(pendingtxHashes, tx.Hash().String())
	}

	return pendingtxHashes, nil, nil
}

// extractTxHashesSorted processes transaction maps in a deterministic order and returns flat slice of tx hashes
func (s *SystemTestSuite) extractTxHashesSorted(txMap map[common.Address][]*ethtypes.Transaction) []string {
	var result []string

	// Get addresses and sort them for deterministic iteration
	addresses := make([]common.Address, 0, len(txMap))
	for addr := range txMap {
		addresses = append(addresses, addr)
	}
	sort.Slice(addresses, func(i, j int) bool {
		return addresses[i].Hex() < addresses[j].Hex()
	})

	// Process addresses in sorted order
	for _, addr := range addresses {
		txs := txMap[addr]

		// Sort transactions by nonce for deterministic ordering
		sort.Slice(txs, func(i, j int) bool {
			return txs[i].Nonce() < txs[j].Nonce()
		})

		// Add transaction hashes to flat result slice
		for _, tx := range txs {
			result = append(result, tx.Hash().String())
		}
	}

	return result
}
