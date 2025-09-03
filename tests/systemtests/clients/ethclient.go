package clients

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/cosmos/evm/tests/systemtests/config"
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

func NewEthClient(config *config.Config) (*EthClient, error) {
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

func (ec *EthClient) RequestArgs(nodeID string, accID string) (context.Context, *ethclient.Client, common.Address) {
	return context.Background(), ec.Clients[nodeID], ec.Accs[accID].Address
}
