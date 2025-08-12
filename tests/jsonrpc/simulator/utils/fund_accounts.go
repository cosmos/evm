package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Standard dev account addresses (matching evmd genesis accounts)
var StandardDevAccounts = map[string]common.Address{
	"dev0": common.HexToAddress("0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101"), // dev0 from local_node.sh
	"dev1": common.HexToAddress("0x963EBDf2e1f8DB8707D05FC75bfeFFBa1B5BaC17"), // dev1 from local_node.sh
	"dev2": common.HexToAddress("0x40a0cb1C63e026A81B55EE1308586E21eec1eFa9"), // dev2 from local_node.sh (CORRECTED)
	"dev3": common.HexToAddress("0x498B5AeC5D439b733dC2F58AB489783A23FB26dA"), // dev3 from local_node.sh (CORRECTED)
}

// Standard dev account balance (1000 ETH = 1000 * 10^18 wei)
var StandardDevBalance = new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18))

// FundingResult holds information about a funding transaction
type FundingResult struct {
	Account string         `json:"account"`
	Address common.Address `json:"address"`
	Amount  *big.Int       `json:"amount"`
	TxHash  common.Hash    `json:"txHash"`
	Success bool           `json:"success"`
	Error   string         `json:"error,omitempty"`
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
	JSONRPC string           `json:"jsonrpc"`
	Result  json.RawMessage  `json:"result"`
	Error   *json.RawMessage `json:"error"`
	ID      int              `json:"id"`
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

// fundStandardAccounts sends funds from geth coinbase to standard dev accounts
func fundStandardAccounts(rCtx *types.RPCContext, isGeth bool) ([]FundingResult, error) {
	results := make([]FundingResult, 0, len(StandardDevAccounts))

	// Get coinbase account (first account from eth_accounts)
	accounts, err := GetAccounts(rCtx, isGeth)
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
		txHash, err := SendTransaction(rCtx, coinbase, address.Hex(), StandardDevBalance, isGeth)
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
