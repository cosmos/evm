package utils

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// TransactionScenario represents a test transaction scenario
type TransactionScenario struct {
	Name        string
	Description string
	TxType      string // "transfer", "erc20_transfer", "erc20_approve", "contract_call"
	FromKey     string // private key
	To          *common.Address
	Value       *big.Int
	Data        []byte
	GasLimit    uint64
	ExpectFail  bool
}

// TransactionResult holds the result of a transaction execution
type TransactionResult struct {
	Scenario    *TransactionScenario
	Network     string // "evmd" or "geth"
	TxHash      common.Hash
	Receipt     *types.Receipt
	Success     bool
	Error       string
	GasUsed     uint64
	BlockNumber *big.Int
	Timestamp   time.Time
}

// TransactionBatch represents a batch of transactions to be executed
type TransactionBatch struct {
	Name         string
	Scenarios    []*TransactionScenario
	EvmdResults  []*TransactionResult
	GethResults  []*TransactionResult
	EvmdContract common.Address
	GethContract common.Address
}

// Dev account private keys (from local_node.sh)
const (
	Dev0PrivateKey = "88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305" // dev0
	Dev1PrivateKey = "741de4f8988ea941d3ff0287911ca4074e62b7d45c991a51186455366f10b544" // dev1  
	Dev2PrivateKey = "3b7955d25189c99a7468192fcbc6429205c158834053ebe3f78f4512ab432db9" // dev2
	Dev3PrivateKey = "8a36c69d940a92fcea94b36d0f2928c7a0ee19a90073eda769693298dfa9603b" // dev3
)

// GetPrivateKeyAndAddress returns the private key and address for a given private key string
func GetPrivateKeyAndAddress(privateKeyHex string) (*ecdsa.PrivateKey, common.Address, error) {
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
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

// CreateTransactionScenarios creates a comprehensive set of transaction test scenarios
func CreateTransactionScenarios(evmdContractAddr, gethContractAddr common.Address) []*TransactionScenario {
	_, dev0Addr, _ := GetPrivateKeyAndAddress(Dev0PrivateKey)
	_, dev1Addr, _ := GetPrivateKeyAndAddress(Dev1PrivateKey)
	_, dev2Addr, _ := GetPrivateKeyAndAddress(Dev2PrivateKey)
	_, dev3Addr, _ := GetPrivateKeyAndAddress(Dev3PrivateKey)

	// ERC20 transfer function signature: transfer(address,uint256)
	transferSig := crypto.Keccak256([]byte("transfer(address,uint256)"))[:4]
	
	// ERC20 approve function signature: approve(address,uint256)
	approveSig := crypto.Keccak256([]byte("approve(address,uint256)"))[:4]

	scenarios := []*TransactionScenario{
		// 1. Simple ETH transfers
		{
			Name:        "eth_transfer_1",
			Description: "Transfer 1 ETH from dev1 to dev2",
			TxType:      "transfer",
			FromKey:     Dev1PrivateKey,
			To:          &dev2Addr,
			Value:       big.NewInt(1000000000000000000), // 1 ETH
			Data:        nil,
			GasLimit:    21000,
			ExpectFail:  false,
		},
		{
			Name:        "eth_transfer_2",
			Description: "Transfer 0.5 ETH from dev2 to dev3",
			TxType:      "transfer",
			FromKey:     Dev2PrivateKey,
			To:          &dev3Addr,
			Value:       big.NewInt(500000000000000000), // 0.5 ETH
			Data:        nil,
			GasLimit:    21000,
			ExpectFail:  false,
		},
		{
			Name:        "eth_transfer_3",
			Description: "Transfer 2 ETH from dev3 to dev0",
			TxType:      "transfer",
			FromKey:     Dev3PrivateKey,
			To:          &dev0Addr,
			Value:       big.NewInt(2000000000000000000), // 2 ETH
			Data:        nil,
			GasLimit:    21000,
			ExpectFail:  false,
		},

		// 2. ERC20 token transfers (will be set dynamically per network)
		{
			Name:        "erc20_transfer_1",
			Description: "Transfer 100 tokens from dev1 to dev2",
			TxType:      "erc20_transfer",
			FromKey:     Dev1PrivateKey,
			To:          &evmdContractAddr, // Will be updated per network
			Value:       big.NewInt(0),
			Data:        buildERC20TransferData(transferSig, dev2Addr, big.NewInt(100)),
			GasLimit:    100000,
			ExpectFail:  false,
		},
		{
			Name:        "erc20_approve_1",
			Description: "Approve dev3 to spend 50 tokens from dev2",
			TxType:      "erc20_approve",
			FromKey:     Dev2PrivateKey,
			To:          &evmdContractAddr, // Will be updated per network
			Value:       big.NewInt(0),
			Data:        buildERC20ApproveData(approveSig, dev3Addr, big.NewInt(50)),
			GasLimit:    100000,
			ExpectFail:  false,
		},
		{
			Name:        "erc20_transfer_2",
			Description: "Transfer 25 tokens from dev2 to dev0",
			TxType:      "erc20_transfer",
			FromKey:     Dev2PrivateKey,
			To:          &evmdContractAddr, // Will be updated per network
			Value:       big.NewInt(0),
			Data:        buildERC20TransferData(transferSig, dev0Addr, big.NewInt(25)),
			GasLimit:    100000,
			ExpectFail:  false,
		},

		// 3. Failed transactions (for testing error handling)
		{
			Name:        "eth_transfer_insufficient_balance",
			Description: "Try to transfer 10000 ETH (should fail - insufficient balance)",
			TxType:      "transfer",
			FromKey:     Dev0PrivateKey,
			To:          &dev1Addr,
			Value:       new(big.Int).Mul(big.NewInt(10000), big.NewInt(1000000000000000000)), // 10000 ETH
			Data:        nil,
			GasLimit:    21000,
			ExpectFail:  true,
		},
		{
			Name:        "erc20_transfer_insufficient_tokens",
			Description: "Try to transfer 1000 tokens (should fail - insufficient balance)",
			TxType:      "erc20_transfer",
			FromKey:     Dev0PrivateKey,
			To:          &evmdContractAddr, // Will be updated per network
			Value:       big.NewInt(0),
			Data:        buildERC20TransferData(transferSig, dev1Addr, big.NewInt(1000)),
			GasLimit:    100000,
			ExpectFail:  true,
		},
	}

	return scenarios
}

// buildERC20TransferData builds the data payload for ERC20 transfer function call
func buildERC20TransferData(transferSig []byte, to common.Address, amount *big.Int) []byte {
	data := make([]byte, 68) // 4 bytes signature + 32 bytes address + 32 bytes amount
	
	// Function signature
	copy(data[0:4], transferSig)
	
	// Recipient address (left-padded to 32 bytes)
	copy(data[16:36], to.Bytes())
	
	// Amount (32 bytes)
	amountBytes := amount.Bytes()
	copy(data[68-len(amountBytes):68], amountBytes)
	
	return data
}

// buildERC20ApproveData builds the data payload for ERC20 approve function call
func buildERC20ApproveData(approveSig []byte, spender common.Address, amount *big.Int) []byte {
	data := make([]byte, 68) // 4 bytes signature + 32 bytes address + 32 bytes amount
	
	// Function signature
	copy(data[0:4], approveSig)
	
	// Spender address (left-padded to 32 bytes)
	copy(data[16:36], spender.Bytes())
	
	// Amount (32 bytes)
	amountBytes := amount.Bytes()
	copy(data[68-len(amountBytes):68], amountBytes)
	
	return data
}

// ExecuteTransactionScenario executes a single transaction scenario on a network
func ExecuteTransactionScenario(client *ethclient.Client, scenario *TransactionScenario, network string) (*TransactionResult, error) {
	result := &TransactionResult{
		Scenario:  scenario,
		Network:   network,
		Timestamp: time.Now(),
	}

	// Get private key and address
	privateKey, fromAddr, err := GetPrivateKeyAndAddress(scenario.FromKey)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get private key: %v", err)
		return result, err
	}

	ctx := context.Background()

	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get chain ID: %v", err)
		return result, err
	}

	// Get nonce
	nonce, err := client.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get nonce: %v", err)
		return result, err
	}

	// Get gas pricing
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get gas price: %v", err)
		return result, err
	}

	// Create transaction
	var tx *types.Transaction
	if scenario.To != nil {
		tx = types.NewTransaction(nonce, *scenario.To, scenario.Value, scenario.GasLimit, gasPrice, scenario.Data)
	} else {
		tx = types.NewContractCreation(nonce, scenario.Value, scenario.GasLimit, gasPrice, scenario.Data)
	}

	// Sign transaction
	signer := types.NewEIP155Signer(chainID)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	if err != nil {
		result.Error = fmt.Sprintf("failed to sign transaction: %v", err)
		return result, err
	}

	// Send transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		if scenario.ExpectFail {
			result.Success = true // Expected to fail
			result.Error = fmt.Sprintf("expected failure: %v", err)
		} else {
			result.Error = fmt.Sprintf("failed to send transaction: %v", err)
		}
		return result, err
	}

	result.TxHash = signedTx.Hash()

	// Wait for transaction to be mined
	receipt, err := waitForTransactionReceipt(client, result.TxHash, 30*time.Second)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get receipt: %v", err)
		return result, err
	}

	result.Receipt = receipt
	result.GasUsed = receipt.GasUsed
	result.BlockNumber = receipt.BlockNumber

	// Check transaction status
	if receipt.Status == 1 {
		if scenario.ExpectFail {
			result.Success = false
			result.Error = "transaction succeeded but was expected to fail"
		} else {
			result.Success = true
		}
	} else {
		if scenario.ExpectFail {
			result.Success = true
			result.Error = "expected failure - transaction reverted"
		} else {
			result.Success = false
			result.Error = "transaction failed - status 0"
		}
	}

	return result, nil
}

// waitForTransactionReceipt waits for a transaction receipt
func waitForTransactionReceipt(client *ethclient.Client, txHash common.Hash, timeout time.Duration) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for transaction %s", txHash.Hex())
		case <-ticker.C:
			receipt, err := client.TransactionReceipt(context.Background(), txHash)
			if err != nil {
				continue // Transaction not mined yet
			}
			return receipt, nil
		}
	}
}