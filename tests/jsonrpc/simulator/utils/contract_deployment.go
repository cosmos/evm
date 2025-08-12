package utils

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

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

// GetDev0PrivateKeyAndAddress returns dev0's private key and address for contract deployment
func GetDev0PrivateKeyAndAddress() (*ecdsa.PrivateKey, common.Address, error) {
	privateKey, err := crypto.HexToECDSA(config.Dev0PrivateKey)
	if err != nil {
		return nil, common.Address{}, err
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, common.Address{}, fmt.Errorf("error casting public key to ECDSA")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	return privateKey, address, nil
}

func deployContractViaDynamicFeeTx(client *ethclient.Client, privateKey *ecdsa.PrivateKey, contractByteCode []byte) (string, error) {
	ctx := context.Background()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return "", err
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("error casting public key to ECDSA")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	nonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return "", err
	}

	maxPriorityFeePerGas, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return "", err
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return "", err
	}

	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: maxPriorityFeePerGas,
		GasFeeCap: new(big.Int).Add(gasPrice, big.NewInt(1000000000)),
		Gas:       10000000,
		Data:      contractByteCode,
	})

	signer := ethtypes.NewLondonSigner(chainID)
	signedTx, err := ethtypes.SignTx(tx, signer, privateKey)
	if err != nil {
		return "", err
	}

	if err = client.SendTransaction(ctx, signedTx); err != nil {
		return "", err
	}

	return signedTx.Hash().Hex(), nil
}

// waitForContractDeployment waits for a deployment transaction to be mined and returns the contract address
func waitForContractDeployment(client *ethclient.Client, txHashStr string, timeout time.Duration) (common.Address, *big.Int, error) {
	fmt.Printf("Waiting for evmd deployment (tx: %s)...\n", txHashStr)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	txHash := common.HexToHash(txHashStr)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return common.Address{}, nil, fmt.Errorf("timeout waiting for deployment transaction %s", txHashStr)
		case <-ticker.C:
			receipt, err := client.TransactionReceipt(context.Background(), txHash)
			if err != nil {
				continue // Transaction not mined yet
			}

			if receipt.Status == 0 {
				return common.Address{}, nil, fmt.Errorf("deployment transaction failed: %s", txHashStr)
			}

			if receipt.ContractAddress == (common.Address{}) {
				return common.Address{}, nil, fmt.Errorf("no contract address in receipt for tx: %s", txHashStr)
			}

			return receipt.ContractAddress, receipt.BlockNumber, nil
		}
	}
}

func DeployERC20Contract(rCtx *types.RPCContext, contractByteCode []byte, isGeth bool) (addr common.Address, txHash string, blockNum *big.Int, err error) {
	ethCli := rCtx.Evmd
	if isGeth {
		ethCli = rCtx.Geth
	}

	privateKey, fromAddress, err := GetDev0PrivateKeyAndAddress()
	if err != nil {
		return common.Address{}, "", nil, fmt.Errorf("failed to get dev0 credentials: %v", err)
	}

	fmt.Printf("Deploying ERC20 to evmd using dev0 (%s)...\n", fromAddress.Hex())

	evmdTxHash, err := deployContractViaDynamicFeeTx(ethCli.Client, privateKey, contractByteCode)
	if err != nil {
		return common.Address{}, "", nil, err
	} else {
		addr, blockNum, err = waitForContractDeployment(ethCli.Client, evmdTxHash, 30*time.Second)
		if err != nil {
			return common.Address{}, "", nil, err
		}
	}

	fmt.Printf("✓ evmd deployment successful: %s\n", addr.Hex())
	return addr, evmdTxHash, blockNum, nil
}
