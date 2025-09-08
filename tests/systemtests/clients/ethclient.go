package clients

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type EthClient struct {
	ChainID *big.Int
	Clients map[string]*ethclient.Client
	Accs    map[string]*EthAccount
}

func NewEthClient() (*EthClient, error) {
	config, err := NewConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config")
	}

	clients := make(map[string]*ethclient.Client, 0)
	for i, jsonrpcUrl := range config.JsonRPCUrls {
		ethcli, err := ethclient.Dial(jsonrpcUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to connecting node url: %s", jsonrpcUrl)
		}
		clients[fmt.Sprintf("node%v", i)] = ethcli
	}

	accs := make(map[string]*EthAccount, 0)
	for i, privKey := range config.PrivKeys {
		ecdsaPrivKey, err := crypto.HexToECDSA(privKey)
		if err != nil {
			return nil, err
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
		Accs:    accs,
	}, nil
}

func (ec *EthClient) Setup(nodeID string, accID string) (context.Context, *ethclient.Client, common.Address) {
	return context.Background(), ec.Clients[nodeID], ec.Accs[accID].Address
}

func (ec *EthClient) SendRawTransaction(
	nodeID string,
	accID string,
	tx *ethtypes.Transaction,
) (common.Hash, error) {
	ethCli := ec.Clients[nodeID]
	privKey := ec.Accs[accID].PrivKey

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

func (ec *EthClient) CheckPendingOrCommited(
	nodeID string,
	txHash string,
	timeout time.Duration,
) error {
	ethCli := ec.Clients[nodeID]

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for transaction %s", txHash)
		case <-ticker.C:
			pendingTxs, _, err := ec.TxPoolContent(nodeID)
			if err != nil {
				return fmt.Errorf("failed to get txpool content")
			}

			pendingTxHashes := extractTxHashesSorted(pendingTxs)

			if ok := slices.Contains(pendingTxHashes, txHash); ok {
				return nil
			}

			if _, err = ethCli.TransactionReceipt(context.Background(), common.HexToHash(txHash)); err == nil {
				return nil
			}
		}
	}
}

func (ec *EthClient) TxPoolContent(nodeID string) (map[string]map[string]*RPCTransaction, map[string]map[string]*RPCTransaction, error) {
	ethCli := ec.Clients[nodeID]

	var result TxPoolResult
	err := ethCli.Client().Call(&result, "txpool_content")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to call txpool_content eth api: %v", err)
	}

	return result.Pending, result.Queued, nil
}

func (ec *EthClient) TxPoolContentFrom(nodeID, accID string) (map[string]map[string]*RPCTransaction, map[string]map[string]*RPCTransaction, error) {
	ethCli := ec.Clients[nodeID]
	addr := ec.Accs[accID].Address

	var result TxPoolResult
	err := ethCli.Client().Call(&result, "txpool_contentFrom", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to call txpool_contentFrom eth api: %v", err)
	}

	return result.Pending, result.Queued, nil
}

// extractTxHashesSorted processes transaction maps in a deterministic order and returns flat slice of tx hashes
func extractTxHashesSorted(txMap map[string]map[string]*RPCTransaction) []string {
	var result []string

	// Get addresses and sort them for deterministic iteration
	addresses := make([]string, 0, len(txMap))
	for addr := range txMap {
		addresses = append(addresses, addr)
	}
	slices.Sort(addresses)

	// Process addresses in sorted order
	for _, addr := range addresses {
		txs := txMap[addr]

		// Sort transactions by nonce for deterministic ordering
		nonces := make([]string, 0, len(txs))
		for nonce := range txs {
			nonces = append(nonces, nonce)
		}
		slices.Sort(nonces)

		// Add transaction hashes to flat result slice
		for _, nonce := range nonces {
			result = append(result, txs[nonce].Hash.Hex())
		}
	}

	return result
}
