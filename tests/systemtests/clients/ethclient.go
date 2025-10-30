package clients

import (
	"context"
	"fmt"
	"maps"
	"math/big"
	"slices"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// EthClient is a client for interacting with Ethereum-compatible nodes.
type EthClient struct {
	ChainID *big.Int
	Clients map[string]*ethclient.Client
}

// NewEthClient creates a new EthClient instance and returns it together with the loaded accounts.
func NewEthClient() (*EthClient, map[string]*EthAccount, error) {
	config, err := NewConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config")
	}

	clients := make(map[string]*ethclient.Client, 0)
	for i, jsonrpcUrl := range config.JsonRPCUrls {
		ethcli, err := ethclient.Dial(jsonrpcUrl)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connecting node url: %s", jsonrpcUrl)
		}
		clients[fmt.Sprintf("node%v", i)] = ethcli
	}

	accs := make(map[string]*EthAccount, 0)
	for i, privKey := range config.PrivKeys {
		ecdsaPrivKey, err := crypto.HexToECDSA(privKey)
		if err != nil {
			return nil, nil, err
		}
		address := crypto.PubkeyToAddress(ecdsaPrivKey.PublicKey)
		acc := &EthAccount{
			Address: address,
			PrivKey: ecdsaPrivKey,
		}
		accs[fmt.Sprintf("acc%v", i)] = acc
	}

	return &EthClient{
		ChainID: config.EVMChainID,
		Clients: clients,
	}, accs, nil
}

// Setup prepares the context, client, and address for the given node and account.
func (ec *EthClient) Setup(nodeID string, account *EthAccount) (context.Context, *ethclient.Client, common.Address) {
	return context.Background(), ec.Clients[nodeID], account.Address
}

// SendRawTransaction sends a raw Ethereum transaction to the specified node.
func (ec *EthClient) SendRawTransaction(
	nodeID string,
	account *EthAccount,
	tx *ethtypes.Transaction,
) (common.Hash, error) {
	ethCli := ec.Clients[nodeID]
	privKey := account.PrivKey

	signer := ethtypes.NewLondonSigner(ec.ChainID)
	signedTx, err := ethtypes.SignTx(tx, signer, privKey)
	if err != nil {
		return common.Hash{}, err
	}

	if err = ethCli.SendTransaction(context.Background(), signedTx); err != nil {
		return common.Hash{}, err
	}

	return signedTx.Hash(), nil
}

// WaitForCommit waits for a transaction to be committed in a block.
func (ec *EthClient) WaitForCommit(
	nodeID string,
	txHash string,
	timeout time.Duration,
) (*ethtypes.Receipt, error) {
	ethCli := ec.Clients[nodeID]

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for transaction %s", txHash)
		case <-ticker.C:
			receipt, err := ethCli.TransactionReceipt(context.Background(), common.HexToHash(txHash))
			if err != nil {
				continue // Transaction not mined yet
			}

			return receipt, nil
		}
	}
}

// CheckTxsPending checks if a transaction is either pending in the mempool or already committed.
func (ec *EthClient) CheckTxsPending(
	nodeID string,
	txHash string,
	timeout time.Duration,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for transaction %s", txHash)
		case <-ticker.C:
			pendingTxs, _, err := ec.TxPoolContent(ctx, nodeID)
			if err != nil {
				fmt.Printf("DEBUG: failed to get txpool content: %v", err)
				continue // Retry on error
			}

			pendingTxHashes := extractTxHashesSorted(pendingTxs)

			if ok := slices.Contains(pendingTxHashes, txHash); ok {
				return nil
			}
		}
	}
}

// TxPoolContent returns the pending and queued tx hashes in the tx pool of the given node
func (ec *EthClient) TxPoolContent(ctx context.Context, nodeID string) (map[string]map[string]*EthRPCTransaction, map[string]map[string]*EthRPCTransaction, error) {
	ethCli := ec.Clients[nodeID]

	var result TxPoolResult
	err := ethCli.Client().CallContext(ctx, &result, "txpool_content")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to call txpool_content eth api: %v", err)
	}

	return result.Pending, result.Queued, nil
}

// extractTxHashesSorted processes transaction maps in a deterministic order and returns flat slice of tx hashes
func extractTxHashesSorted(txMap map[string]map[string]*EthRPCTransaction) []string {
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

func (ec *EthClient) CodeAt(nodeID string, account *EthAccount) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	blockNumber, err := ec.Clients[nodeID].BlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query block number: %w", err)
	}

	code, err := ec.Clients[nodeID].CodeAt(ctx, account.Address, big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, fmt.Errorf("failed to query code for %s: %w", account.Address.Hex(), err)
	}

	return code, nil
}
