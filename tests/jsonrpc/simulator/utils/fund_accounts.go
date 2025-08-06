package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Standard dev account addresses (matching evmd genesis accounts)
var StandardDevAccounts = map[string]common.Address{
	"dev1": common.HexToAddress("0x963EBDf2e1f8DB8707D05FC75bfeFFBa1B5BaC17"), // dev1 from local_node.sh
	"dev2": common.HexToAddress("0x40B5A72A98b3dD51C6EEdCF7c2078671a08B4e2F"), // dev2 from local_node.sh  
	"dev3": common.HexToAddress("0xF44eF20ED88eFdC8e68e74b9bF51ffCb4b6A1415"), // dev3 from local_node.sh
	"dev4": common.HexToAddress("0x742F5b99D4D3d9FB5E8C2b6C6BfE5C61DD2f10dc"), // dev4 from local_node.sh
}

// Standard dev account balance (1000 ETH = 1000 * 10^18 wei)
var StandardDevBalance = new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18))

// FundingResult holds information about a funding transaction
type FundingResult struct {
	Account     string      `json:"account"`
	Address     common.Address `json:"address"`
	Amount      *big.Int    `json:"amount"`
	TxHash      common.Hash `json:"txHash"`
	Success     bool        `json:"success"`
	Error       string      `json:"error,omitempty"`
}

// JSONRPCRequest represents a JSON-RPC request
type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// JSONRPCResponse represents a JSON-RPC response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *json.RawMessage `json:"error"`
	ID      int             `json:"id"`
}

// GetGethAccounts gets accounts from geth using JSON-RPC
func GetGethAccounts(rpcURL string) ([]common.Address, error) {
	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_accounts",
		Params:  []interface{}{},
		ID:      1,
	}

	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(rpcURL, "application/json", strings.NewReader(string(reqData)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s", string(*rpcResp.Error))
	}

	var accountStrings []string
	if err := json.Unmarshal(rpcResp.Result, &accountStrings); err != nil {
		return nil, err
	}

	accounts := make([]common.Address, len(accountStrings))
	for i, accountStr := range accountStrings {
		accounts[i] = common.HexToAddress(accountStr)
	}

	return accounts, nil
}

// SendTransactionRequest represents the eth_sendTransaction request parameters
type SendTransactionRequest struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Value string `json:"value"`
	Gas   string `json:"gas"`
}

// sendEthTransaction sends a transaction using eth_sendTransaction (for unlocked accounts)
func sendEthTransaction(rpcURL string, from, to common.Address, value *big.Int) (string, error) {
	// Create transaction request
	txReq := SendTransactionRequest{
		From:  from.Hex(),
		To:    to.Hex(),
		Value: fmt.Sprintf("0x%x", value), // Convert to hex string
		Gas:   "0x5208",                   // 21000 gas for simple transfer
	}

	// Create JSON-RPC request
	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_sendTransaction",
		Params:  []interface{}{txReq},
		ID:      1,
	}

	reqData, err := json.Marshal(rpcReq)
	if err != nil {
		return "", err
	}

	// Send request
	resp, err := http.Post(rpcURL, "application/json", bytes.NewReader(reqData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Parse response
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

// FundStandardAccounts sends funds from geth coinbase to standard dev accounts
func FundStandardAccounts(client *ethclient.Client, rpcURL string) ([]FundingResult, error) {
	results := make([]FundingResult, 0, len(StandardDevAccounts))

	// Get coinbase account (first account from eth_accounts)
	accounts, err := GetGethAccounts(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no accounts found in geth")
	}

	coinbase := accounts[0] // First account is coinbase in dev mode

	// Fund each standard dev account using eth_sendTransaction
	for name, address := range StandardDevAccounts {
		result := FundingResult{
			Account: name,
			Address: address,
			Amount:  StandardDevBalance,
		}

		// Send transaction using eth_sendTransaction (coinbase is unlocked in dev mode)
		txHash, err := sendEthTransaction(rpcURL, coinbase, address, StandardDevBalance)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Success = true
			result.TxHash = common.HexToHash(txHash)
		}

		results = append(results, result)
	}

	return results, nil
}

// CheckAccountBalances verifies that accounts have the expected balances
func CheckAccountBalances(client *ethclient.Client) (map[string]*big.Int, error) {
	ctx := context.Background()
	balances := make(map[string]*big.Int)

	for name, address := range StandardDevAccounts {
		balance, err := client.BalanceAt(ctx, address, nil)
		if err != nil {
			return nil, err
		}
		balances[name] = balance
	}

	return balances, nil
}