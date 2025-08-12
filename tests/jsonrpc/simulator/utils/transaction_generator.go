package utils

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
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

// TransactionMetadata holds comprehensive transaction information for API testing
type TransactionMetadata struct {
	Scenario    *TransactionScenario `json:"scenario"`
	Network     string               `json:"network"` // "evmd" or "geth"
	TxHash      common.Hash          `json:"txHash"`
	Receipt     *ethtypes.Receipt    `json:"receipt,omitempty"`
	Success     bool                 `json:"success"`
	Error       string               `json:"error,omitempty"`
	GasUsed     uint64               `json:"gasUsed"`
	BlockNumber *big.Int             `json:"blockNumber,omitempty"`
	Timestamp   time.Time            `json:"timestamp"`

	// Enhanced metadata for API testing
	Transaction     *ethtypes.Transaction `json:"transaction,omitempty"`     // Original transaction
	TransactionRaw  string                `json:"transactionRaw,omitempty"`  // RLP-encoded transaction
	ReceiptRaw      string                `json:"receiptRaw,omitempty"`      // JSON-encoded receipt
	Logs            []*ethtypes.Log       `json:"logs,omitempty"`            // Transaction logs
	ContractAddress *common.Address       `json:"contractAddress,omitempty"` // For contract deployments
	RevertReason    string                `json:"revertReason,omitempty"`    // Decoded revert reason

	// API call metadata
	JSONRPCCalls   []JSONRPCCallInfo `json:"jsonrpcCalls,omitempty"` // All JSON-RPC calls made
	APICallLatency time.Duration     `json:"apiCallLatency"`         // Total API call time
}

// JSONRPCCallInfo tracks individual JSON-RPC method calls
type JSONRPCCallInfo struct {
	Method    string        `json:"method"`
	Params    interface{}   `json:"params,omitempty"`
	Response  interface{}   `json:"response,omitempty"`
	Error     string        `json:"error,omitempty"`
	Latency   time.Duration `json:"latency"`
	Timestamp time.Time     `json:"timestamp"`
}

// TransactionResult holds the result of a transaction execution (legacy compatibility)
type TransactionResult struct {
	Scenario    *TransactionScenario
	Network     string // "evmd" or "geth"
	TxHash      common.Hash
	Receipt     *ethtypes.Receipt
	Success     bool
	Error       string
	GasUsed     uint64
	BlockNumber *big.Int
	Timestamp   time.Time
}

// ToTransactionMetadata converts TransactionResult to enhanced TransactionMetadata
func (tr *TransactionResult) ToTransactionMetadata() *TransactionMetadata {
	return &TransactionMetadata{
		Scenario:     tr.Scenario,
		Network:      tr.Network,
		TxHash:       tr.TxHash,
		Receipt:      tr.Receipt,
		Success:      tr.Success,
		Error:        tr.Error,
		GasUsed:      tr.GasUsed,
		BlockNumber:  tr.BlockNumber,
		Timestamp:    tr.Timestamp,
		Logs:         tr.Receipt.Logs,
		JSONRPCCalls: make([]JSONRPCCallInfo, 0),
	}
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

// TransactionMetadataBatch represents a batch with enhanced metadata
type TransactionMetadataBatch struct {
	Name         string                 `json:"name"`
	Scenarios    []*TransactionScenario `json:"scenarios"`
	EvmdResults  []*TransactionMetadata `json:"evmdResults"`
	GethResults  []*TransactionMetadata `json:"gethResults"`
	EvmdContract common.Address         `json:"evmdContract"`
	GethContract common.Address         `json:"gethContract"`

	// Batch-level metadata
	StartTime    time.Time     `json:"startTime"`
	EndTime      time.Time     `json:"endTime"`
	TotalLatency time.Duration `json:"totalLatency"`
	SuccessCount int           `json:"successCount"`
	FailureCount int           `json:"failureCount"`
}

// NewTransactionMetadataBatch creates a new batch with enhanced metadata
func NewTransactionMetadataBatch(name string, scenarios []*TransactionScenario, evmdAddr, gethAddr common.Address) *TransactionMetadataBatch {
	return &TransactionMetadataBatch{
		Name:         name,
		Scenarios:    scenarios,
		EvmdResults:  make([]*TransactionMetadata, 0, len(scenarios)),
		GethResults:  make([]*TransactionMetadata, 0, len(scenarios)),
		EvmdContract: evmdAddr,
		GethContract: gethAddr,
		StartTime:    time.Now(),
	}
}

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
	_, dev0Addr, _ := GetPrivateKeyAndAddress(config.Dev0PrivateKey)
	_, dev1Addr, _ := GetPrivateKeyAndAddress(config.Dev1PrivateKey)
	_, dev2Addr, _ := GetPrivateKeyAndAddress(config.Dev2PrivateKey)
	_, dev3Addr, _ := GetPrivateKeyAndAddress(config.Dev3PrivateKey)

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
			FromKey:     config.Dev1PrivateKey,
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
			FromKey:     config.Dev2PrivateKey,
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
			FromKey:     config.Dev3PrivateKey,
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
			FromKey:     config.Dev1PrivateKey,
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
			FromKey:     config.Dev2PrivateKey,
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
			FromKey:     config.Dev2PrivateKey,
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
			FromKey:     config.Dev0PrivateKey,
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
			FromKey:     config.Dev0PrivateKey,
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

// ExecuteTransactionScenarioWithMetadata executes a transaction scenario and captures detailed metadata
func ExecuteTransactionScenarioWithMetadata(client *ethclient.Client, rpcURL string, scenario *TransactionScenario, network string) (*TransactionMetadata, error) {
	startTime := time.Now()
	metadata := &TransactionMetadata{
		Scenario:     scenario,
		Network:      network,
		Timestamp:    startTime,
		JSONRPCCalls: make([]JSONRPCCallInfo, 0),
	}

	// Get private key and address
	privateKey, fromAddr, err := GetPrivateKeyAndAddress(scenario.FromKey)
	if err != nil {
		metadata.Error = fmt.Sprintf("failed to get private key: %v", err)
		return metadata, err
	}

	ctx := context.Background()

	// Fund account if this is geth and insufficient balance
	if network == "geth" {
		if err := fundGethAccountIfNeeded(client, fromAddr); err != nil {
			metadata.Error = fmt.Sprintf("failed to fund geth account: %v", err)
			return metadata, err
		}
	}

	// Track chain ID call
	callStart := time.Now()
	chainID, err := client.ChainID(ctx)
	if err != nil {
		metadata.Error = fmt.Sprintf("failed to get chain ID: %v", err)
		return metadata, err
	}
	metadata.JSONRPCCalls = append(metadata.JSONRPCCalls, JSONRPCCallInfo{
		Method:    "eth_chainId",
		Response:  chainID.String(),
		Latency:   time.Since(callStart),
		Timestamp: callStart,
	})

	// Track nonce call
	callStart = time.Now()
	nonce, err := client.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		metadata.Error = fmt.Sprintf("failed to get nonce: %v", err)
		return metadata, err
	}
	metadata.JSONRPCCalls = append(metadata.JSONRPCCalls, JSONRPCCallInfo{
		Method:    "eth_getTransactionCount",
		Params:    map[string]interface{}{"address": fromAddr.Hex(), "block": "pending"},
		Response:  nonce,
		Latency:   time.Since(callStart),
		Timestamp: callStart,
	})

	// Track gas price call
	callStart = time.Now()
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		metadata.Error = fmt.Sprintf("failed to get gas price: %v", err)
		return metadata, err
	}
	metadata.JSONRPCCalls = append(metadata.JSONRPCCalls, JSONRPCCallInfo{
		Method:    "eth_gasPrice",
		Response:  gasPrice.String(),
		Latency:   0,
		Timestamp: callStart,
	})

	// Create transaction
	var tx *ethtypes.Transaction
	if scenario.To != nil {
		tx = ethtypes.NewTransaction(nonce, *scenario.To, scenario.Value, scenario.GasLimit, gasPrice, scenario.Data)
	} else {
		tx = ethtypes.NewContractCreation(nonce, scenario.Value, scenario.GasLimit, gasPrice, scenario.Data)
	}

	// Sign transaction
	signer := ethtypes.NewEIP155Signer(chainID)
	signedTx, err := ethtypes.SignTx(tx, signer, privateKey)
	if err != nil {
		metadata.Error = fmt.Sprintf("failed to sign transaction: %v", err)
		return metadata, err
	}

	// Store transaction details
	metadata.Transaction = signedTx

	// Track send transaction call
	callStart = time.Now()
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		metadata.JSONRPCCalls = append(metadata.JSONRPCCalls, JSONRPCCallInfo{
			Method:    "eth_sendRawTransaction",
			Error:     err.Error(),
			Latency:   time.Since(callStart),
			Timestamp: callStart,
		})

		if scenario.ExpectFail {
			metadata.Success = true
			metadata.Error = fmt.Sprintf("expected failure: %v", err)
		} else {
			metadata.Error = fmt.Sprintf("failed to send transaction: %v", err)
		}
		return metadata, err
	}

	metadata.JSONRPCCalls = append(metadata.JSONRPCCalls, JSONRPCCallInfo{
		Method:    "eth_sendRawTransaction",
		Response:  signedTx.Hash().Hex(),
		Latency:   time.Since(callStart),
		Timestamp: callStart,
	})

	metadata.TxHash = signedTx.Hash()

	// Wait for transaction receipt with detailed tracking
	receipt, err := waitForTransactionReceiptWithTracking(client, metadata.TxHash, 30*time.Second, &metadata.JSONRPCCalls)
	if err != nil {
		metadata.Error = fmt.Sprintf("failed to get receipt: %v", err)
		return metadata, err
	}

	metadata.Receipt = receipt
	metadata.GasUsed = receipt.GasUsed
	metadata.BlockNumber = receipt.BlockNumber
	metadata.Logs = receipt.Logs

	// Extract contract address if it's a deployment
	if receipt.ContractAddress != (common.Address{}) {
		metadata.ContractAddress = &receipt.ContractAddress
	}

	// Check transaction status
	if receipt.Status == 1 {
		if scenario.ExpectFail {
			metadata.Success = false
			metadata.Error = "transaction succeeded but was expected to fail"
		} else {
			metadata.Success = true
		}
	} else {
		if scenario.ExpectFail {
			metadata.Success = true
			metadata.Error = "expected failure - transaction reverted"
		} else {
			metadata.Success = false
			metadata.Error = types.ErrorTansactionFailed
		}
	}

	metadata.APICallLatency = time.Since(startTime)
	return metadata, nil
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
	var tx *ethtypes.Transaction
	if scenario.To != nil {
		tx = ethtypes.NewTransaction(nonce, *scenario.To, scenario.Value, scenario.GasLimit, gasPrice, scenario.Data)
	} else {
		tx = ethtypes.NewContractCreation(nonce, scenario.Value, scenario.GasLimit, gasPrice, scenario.Data)
	}

	// Sign transaction
	signer := ethtypes.NewEIP155Signer(chainID)
	signedTx, err := ethtypes.SignTx(tx, signer, privateKey)
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
			result.Error = types.ErrorTansactionFailed
		}
	}

	return result, nil
}

// waitForTransactionReceiptWithTracking waits for a transaction receipt and tracks JSON-RPC calls
func waitForTransactionReceiptWithTracking(client *ethclient.Client, txHash common.Hash, timeout time.Duration, jsonrpcCalls *[]JSONRPCCallInfo) (*ethtypes.Receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	attempts := 0
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for transaction %s after %d attempts", txHash.Hex(), attempts)
		case <-ticker.C:
			attempts++
			callStart := time.Now()
			receipt, err := client.TransactionReceipt(context.Background(), txHash)

			if err != nil {
				*jsonrpcCalls = append(*jsonrpcCalls, JSONRPCCallInfo{
					Method:    "eth_getTransactionReceipt",
					Params:    txHash.Hex(),
					Error:     err.Error(),
					Latency:   time.Since(callStart),
					Timestamp: callStart,
				})
				continue // Transaction not mined yet
			}

			*jsonrpcCalls = append(*jsonrpcCalls, JSONRPCCallInfo{
				Method:    "eth_getTransactionReceipt",
				Params:    txHash.Hex(),
				Response:  fmt.Sprintf("status=%d, gasUsed=%d", receipt.Status, receipt.GasUsed),
				Latency:   time.Since(callStart),
				Timestamp: callStart,
			})

			return receipt, nil
		}
	}
}

// waitForTransactionReceipt waits for a transaction receipt
func waitForTransactionReceipt(client *ethclient.Client, txHash common.Hash, timeout time.Duration) (*ethtypes.Receipt, error) {
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

// EnhanceTransactionMetadata adds additional computed fields to transaction metadata
func EnhanceTransactionMetadata(metadata *TransactionMetadata) error {
	if metadata.Transaction != nil {
		// Add RLP-encoded transaction
		txBytes, err := rlp.EncodeToBytes(metadata.Transaction)
		if err != nil {
			return fmt.Errorf("failed to RLP encode transaction: %w", err)
		}
		metadata.TransactionRaw = "0x" + hex.EncodeToString(txBytes)
	}

	// Add JSON-encoded receipt if available
	if metadata.Receipt != nil {
		// We don't serialize the entire receipt to avoid circular references
		// Instead, create a summary
		metadata.ReceiptRaw = fmt.Sprintf(`{"status":%d,"gasUsed":%d,"logs":%d,"blockNumber":%s}`,
			metadata.Receipt.Status,
			metadata.Receipt.GasUsed,
			len(metadata.Receipt.Logs),
			metadata.Receipt.BlockNumber.String())
	}

	return nil
}

// fundGethAccountIfNeeded funds a geth account if it has insufficient balance
func fundGethAccountIfNeeded(client *ethclient.Client, targetAddr common.Address) error {
	ctx := context.Background()

	// Check current balance
	balance, err := client.BalanceAt(ctx, targetAddr, nil)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}

	// If balance is sufficient (>= 50 ETH), no need to fund
	minBalance := new(big.Int).Mul(big.NewInt(50), big.NewInt(1e18)) // 50 ETH
	if balance.Cmp(minBalance) >= 0 {
		return nil
	}

	// Silently funding geth account

	// Get geth's pre-funded dev account
	var accounts []string
	err = client.Client().Call(&accounts, "eth_accounts")
	if err != nil || len(accounts) == 0 {
		return fmt.Errorf("no funded accounts available in geth: %w", err)
	}

	devAccount := common.HexToAddress(accounts[0])
	// Using geth dev account for funding

	// Transfer 500 ETH from dev account to target account
	transferAmount := new(big.Int).Mul(big.NewInt(500), big.NewInt(1e18)) // 500 ETH

	// Send transaction via RPC (since geth dev account is unlocked)
	var txHash common.Hash
	err = client.Client().Call(&txHash, "eth_sendTransaction", map[string]interface{}{
		"from":  devAccount.Hex(),
		"to":    targetAddr.Hex(),
		"value": fmt.Sprintf("0x%x", transferAmount),
		"gas":   "0x5208", // 21000 gas
	})

	if err != nil {
		return fmt.Errorf("failed to send funding transaction: %w", err)
	}

	// Wait for the funding transaction to be mined properly
	receipt, err := waitForGethTransactionReceipt(client, txHash, 30*time.Second)
	if err != nil {
		return fmt.Errorf("funding transaction failed: %w", err)
	}

	if receipt.Status == 0 {
		return fmt.Errorf("funding transaction reverted")
	}

	// Successfully funded geth account

	return nil
}

// waitForGethTransactionReceipt waits for a geth transaction receipt with proper error handling
func waitForGethTransactionReceipt(client *ethclient.Client, txHash common.Hash, timeout time.Duration) (*ethtypes.Receipt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	attempts := 0
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for transaction %s after %d attempts", txHash.Hex(), attempts)
		case <-ticker.C:
			attempts++
			receipt, err := client.TransactionReceipt(context.Background(), txHash)
			if err != nil {
				// Transaction not mined yet, continue waiting
				continue
			}
			// Transaction found and mined
			return receipt, nil
		}
	}
}
