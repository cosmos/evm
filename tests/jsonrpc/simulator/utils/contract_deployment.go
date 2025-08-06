package utils

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ContractDeployment represents a deployed contract
type ContractDeployment struct {
	Name        string         `json:"name"`
	Address     common.Address `json:"address"`
	TxHash      common.Hash    `json:"txHash"`
	BlockNumber *big.Int       `json:"blockNumber,omitempty"`
	Network     string         `json:"network"` // "evmd" or "geth"
	DeployedAt  time.Time      `json:"deployedAt"`
	Success     bool           `json:"success"`
	Error       string         `json:"error,omitempty"`
}

// DeploymentResult holds results for both networks
type DeploymentResult struct {
	EvmdDeployment *ContractDeployment `json:"evmdDeployment,omitempty"`
	GethDeployment *ContractDeployment `json:"gethDeployment,omitempty"`
	Success        bool                `json:"success"`
	Error          string              `json:"error,omitempty"`
}

// ContractDeploymentRequest for JSON-RPC
type ContractDeploymentRequest struct {
	From     string `json:"from"`
	Data     string `json:"data"`
	Gas      string `json:"gas"`
	GasPrice string `json:"gasPrice,omitempty"`
	Value    string `json:"value,omitempty"`
}


// Dev1 private key from local_node.sh
const Dev1PrivateKey = "741de4f8988ea941d3ff0287911ca4074e62b7d45c991a51186455366f10b544"

// GetDev1PrivateKeyAndAddress returns dev1's private key and address for contract deployment
func GetDev1PrivateKeyAndAddress() (*ecdsa.PrivateKey, common.Address, error) {
	privateKey, err := crypto.HexToECDSA(Dev1PrivateKey)
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

// deployContractViaRPC deploys a contract using JSON-RPC (for unlocked accounts like geth dev mode)
func deployContractViaRPC(rpcURL, fromAddress, contractBytecode string, gasLimit *big.Int) (string, error) {
	deployReq := ContractDeploymentRequest{
		From: fromAddress,
		Data: "0x" + contractBytecode,
		Gas:  fmt.Sprintf("0x%x", gasLimit),
	}

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_sendTransaction",
		Params:  []any{deployReq},
		ID:      1,
	}

	reqData, err := json.Marshal(rpcReq)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(rpcURL, "application/json", bytes.NewReader(reqData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", err
	}

	if rpcResp.Error != nil {
		return "", fmt.Errorf("RPC error: %s", string(*rpcResp.Error))
	}

	var txHash string
	if err := json.Unmarshal(rpcResp.Result, &txHash); err != nil {
		return "", err
	}

	return txHash, nil
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

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: maxPriorityFeePerGas,
		GasFeeCap: new(big.Int).Add(gasPrice, big.NewInt(1000000000)),
		Gas:       10000000,
		Data:      contractByteCode,
	})

	signer := types.NewLondonSigner(chainID)
	signedTx, err := types.SignTx(tx, signer, privateKey)
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

func DeployERC20Contract(evmdURL, gethURL string, contractByteCode []byte) (*DeploymentResult, error) {
	result := &DeploymentResult{}
	
	evmdClient, err := ethclient.Dial(evmdURL)
	if err != nil {
		result.Error = fmt.Sprintf("failed to connect to evmd: %v", err)
		return result, err
	}

	privateKey, fromAddress, err := GetDev1PrivateKeyAndAddress()
	if err != nil {
		result.Error = fmt.Sprintf("failed to get dev1 credentials: %v", err)
		return result, err
	}

	fmt.Printf("Deploying ERC20 to evmd using dev1 (%s)...\n", fromAddress.Hex())
	
	evmdTxHash, err := deployContractViaDynamicFeeTx(evmdClient, privateKey, contractByteCode)
	if err != nil {
		result.EvmdDeployment = &ContractDeployment{
			Name:       "ERC20",
			Network:    "evmd",
			DeployedAt: time.Now(),
			Success:    false,
			Error:      err.Error(),
		}
	} else {
		fmt.Printf("Waiting for evmd deployment (tx: %s)...\n", evmdTxHash)
		evmdAddress, evmdBlock, err := waitForContractDeployment(evmdClient, evmdTxHash, 30*time.Second)
		if err != nil {
			result.EvmdDeployment = &ContractDeployment{
				Name:       "ERC20",
				TxHash:     common.HexToHash(evmdTxHash),
				Network:    "evmd",
				DeployedAt: time.Now(),
				Success:    false,
				Error:      err.Error(),
			}
		} else {
			result.EvmdDeployment = &ContractDeployment{
				Name:        "ERC20",
				Address:     evmdAddress,
				TxHash:      common.HexToHash(evmdTxHash),
				BlockNumber: evmdBlock,
				Network:     "evmd",
				DeployedAt:  time.Now(),
				Success:     true,
			}
			fmt.Printf("✓ evmd deployment successful: %s\n", evmdAddress.Hex())
		}
	}

	gethClient, err := ethclient.Dial(gethURL)
	if err != nil {
		result.Error = fmt.Sprintf("failed to connect to geth: %v", err)
		return result, err
	}

	accounts, err := GetGethAccounts(gethURL)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get geth accounts: %v", err)
		return result, err
	}

	if len(accounts) == 0 {
		result.Error = "no accounts found in geth"
		return result, fmt.Errorf("no accounts found in geth")
	}

	gethCoinbase := accounts[0]
	fmt.Printf("Deploying ERC20 to geth using coinbase (%s)...\n", gethCoinbase.Hex())

	gethTxHash, err := deployContractViaRPC(gethURL, gethCoinbase.Hex(), hex.EncodeToString(contractByteCode), big.NewInt(10000000))
	if err != nil {
		result.GethDeployment = &ContractDeployment{
			Name:       "ERC20",
			Network:    "geth",
			DeployedAt: time.Now(),
			Success:    false,
			Error:      err.Error(),
		}
	} else {
		fmt.Printf("Waiting for geth deployment (tx: %s)...\n", gethTxHash)
		gethAddress, gethBlock, err := waitForContractDeployment(gethClient, gethTxHash, 30*time.Second)
		if err != nil {
			result.GethDeployment = &ContractDeployment{
				Name:       "ERC20",
				TxHash:     common.HexToHash(gethTxHash),
				Network:    "geth",
				DeployedAt: time.Now(),
				Success:    false,
				Error:      err.Error(),
			}
		} else {
			result.GethDeployment = &ContractDeployment{
				Name:        "ERC20",
				Address:     gethAddress,
				TxHash:      common.HexToHash(gethTxHash),
				BlockNumber: gethBlock,
				Network:     "geth",
				DeployedAt:  time.Now(),
				Success:     true,
			}
			fmt.Printf("✓ geth deployment successful: %s\n", gethAddress.Hex())
		}
	}

	// Check overall success
	result.Success = result.EvmdDeployment != nil && result.EvmdDeployment.Success &&
		result.GethDeployment != nil && result.GethDeployment.Success

	if !result.Success {
		var errors []string
		if result.EvmdDeployment != nil && !result.EvmdDeployment.Success {
			errors = append(errors, fmt.Sprintf("evmd: %s", result.EvmdDeployment.Error))
		}
		if result.GethDeployment != nil && !result.GethDeployment.Success {
			errors = append(errors, fmt.Sprintf("geth: %s", result.GethDeployment.Error))
		}
		result.Error = strings.Join(errors, "; ")
	}

	return result, nil
}