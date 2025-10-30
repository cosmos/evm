package suite

import (
	"context"
	"fmt"
	"maps"
	"math/big"
	"slices"
	"time"

	"github.com/cosmos/evm/tests/systemtests/clients"
)

// NonceAt returns the account nonce for the given account at the latest block
func (s *BaseTestSuite) NonceAt(nodeID string, accID string) (uint64, error) {
	account := s.EthAccount(accID)
	ctx, cli, addr := s.EthClient.Setup(nodeID, account)
	blockNumber, err := s.EthClient.Clients[nodeID].BlockNumber(ctx)
	if err != nil {
		return uint64(0), fmt.Errorf("failed to get block number from %s: %v", nodeID, err)
	}
	if int64(blockNumber) < 0 {
		return uint64(0), fmt.Errorf("invaid block number %d", blockNumber)
	}
	return cli.NonceAt(ctx, addr, big.NewInt(int64(blockNumber)))
}

// GetLatestBaseFee returns the base fee of the latest block
func (s *BaseTestSuite) GetLatestBaseFee(nodeID string) (*big.Int, error) {
	account := s.EthAccount("acc0")
	ctx, cli, _ := s.EthClient.Setup(nodeID, account)
	blockNumber, err := cli.BlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get block number from %s: %v", nodeID, err)
	}
	if int64(blockNumber) < 0 {
		return nil, fmt.Errorf("invaid block number %d", blockNumber)
	}

	block, err := cli.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, fmt.Errorf("failed to get block from %s: %v", nodeID, err)
	}

	if block.BaseFee().Cmp(big.NewInt(0)) <= 0 {
		return nil, fmt.Errorf("failed to get block from %s: %v", nodeID, err)
	}

	return block.BaseFee(), nil
}

// BaseFee returns the base fee of the latest block
func (s *BaseTestSuite) WaitForCommit(
	nodeID string,
	txHash string,
	txType string,
	timeout time.Duration,
) error {
	switch txType {
	case TxTypeEVM:
		return s.waitForEthCommmit(nodeID, txHash, timeout)
	case TxTypeCosmos:
		return s.waitForCosmosCommmit(nodeID, txHash, timeout)
	default:
		return fmt.Errorf("invalid txtype: %s", txType)
	}
}

// waitForEthCommmit waits for the given eth tx to be committed within the timeout duration
func (s *BaseTestSuite) waitForEthCommmit(
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

// waitForCosmosCommmit waits for the given cosmos tx to be committed within the timeout duration
func (s *BaseTestSuite) waitForCosmosCommmit(
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

// CheckTxPending checks if the given tx is pending within the timeout duration
func (s *BaseTestSuite) CheckTxPending(
	nodeID string,
	txHash string,
	txType string,
	timeout time.Duration,
) error {
	switch txType {
	case TxTypeEVM:
		return s.EthClient.CheckTxsPending(nodeID, txHash, timeout)
	case TxTypeCosmos:
		// Note: Cosmos transactions vanish from the mempool right after they get included in a block.
		// CosmosClient.CheckTxsPending therefore treats “pending or already committed” as success,
		// whereas the EVM client keeps transactions in the EVM pool until nonce progression occurs.
		err := s.CosmosClient.CheckTxsPending(nodeID, txHash, timeout)
		if err != nil {
			_, err = s.CosmosClient.WaitForCommit(nodeID, txHash, timeout)
			return err
		}
		return nil

	default:
		return fmt.Errorf("invalid tx type")
	}
}

const defaultTxPoolContentTimeout = 60 * time.Second

// TxPoolContent returns the pending and queued tx hashes in the tx pool of the given node
func (s *BaseTestSuite) TxPoolContent(nodeID string, txType string, timeout time.Duration) (pendingTxs, queuedTxs []string, err error) {
	if timeout <= 0 {
		timeout = defaultTxPoolContentTimeout
	}

	switch txType {
	case TxTypeEVM:
		return s.ethTxPoolContent(nodeID, timeout)
	case TxTypeCosmos:
		return s.cosmosTxPoolContent(nodeID, timeout)
	default:
		return nil, nil, fmt.Errorf("invalid tx type")
	}
}

// ethTxPoolContent returns the pending and queued tx hashes in the tx pool of the given node
func (s *BaseTestSuite) ethTxPoolContent(nodeID string, timeout time.Duration) (pendingTxHashes, queuedTxHashes []string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pendingTxs, queuedTxs, err := s.EthClient.TxPoolContent(ctx, nodeID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get txpool content from eth client: %v", err)
	}

	return s.extractTxHashesSorted(pendingTxs), s.extractTxHashesSorted(queuedTxs), nil
}

// cosmosTxPoolContent returns the pending tx hashes in the tx pool of the given node
func (s *BaseTestSuite) cosmosTxPoolContent(nodeID string, timeout time.Duration) (pendingTxHashes, queuedTxHashes []string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result, err := s.CosmosClient.UnconfirmedTxs(ctx, nodeID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to call unconfired transactions from cosmos client: %v", err)
	}

	pendingtxHashes := make([]string, 0)
	for _, tx := range result.Txs {
		pendingtxHashes = append(pendingtxHashes, string(tx.Hash()))
	}

	return pendingtxHashes, nil, nil
}

// extractTxHashesSorted processes transaction maps in a deterministic order and returns flat slice of tx hashes
func (s *BaseTestSuite) extractTxHashesSorted(txMap map[string]map[string]*clients.EthRPCTransaction) []string {
	var result []string

	// Get addresses and sort them for deterministic iteration
	addresses := slices.Collect(maps.Keys(txMap))
	slices.Sort(addresses)

	// Process addresses in sorted order
	for _, addr := range addresses {
		txs := txMap[addr]

		// Sort transactions by nonce for deterministic ordering
		nonces := slices.Collect(maps.Keys(txs))
		slices.Sort(nonces)

		// Add transaction hashes to flat result slice
		for _, nonce := range nonces {
			result = append(result, txs[nonce].Hash.Hex())
		}
	}

	return result
}
