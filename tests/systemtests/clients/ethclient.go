package clients

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/evmos/tests/systemtests/config"
)

type Account struct {
	Address common.Address
	PrivKey *ecdsa.PrivateKey
}

type EthClient struct {
	ChainID *big.Int
	Clients map[string]*ethclient.Client
	Accs    map[string]*Account
}

type TxType struct {
	To *common.Address
}

func NewEthClient(config *config.Config) (*EthClient, error) {
	clients := make(map[string]*ethclient.Client, 0)
	for i, nodeUrl := range config.NodeUrls {
		ethcli, err := ethclient.Dial(nodeUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to connecting node url: %s", nodeUrl)
		}
		clients[fmt.Sprintf("node%v", i)] = ethcli
	}

	accs := make(map[string]*Account, 0)
	for i, privKey := range config.PrivKeys {
		ecdsaPrivKey, err := crypto.HexToECDSA(privKey)
		if err != nil {
			return nil, err
		}
		address := crypto.PubkeyToAddress(ecdsaPrivKey.PublicKey)
		acc := &Account{
			Address: address,
			PrivKey: ecdsaPrivKey,
		}
		accs[fmt.Sprintf("acc%v", i)] = acc
	}

	return &EthClient{
		ChainID: config.ChainID,
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

func (ec *EthClient) WaitForTransaction(
	nodeID string,
	txHash common.Hash,
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
			return nil, fmt.Errorf("timeout waiting for transaction %s", txHash.Hex())
		case <-ticker.C:
			receipt, err := ethCli.TransactionReceipt(context.Background(), txHash)
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
