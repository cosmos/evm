package rpc

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/contracts"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/utils"
)

// GethVersion is the version of the Geth client used in the tests
// Update it when go-ethereum of go.mod is updated
const GethVersion = "1.15.10"

type CallRPC func(rCtx *RpcContext) (*types.RpcResult, error)

type RpcContext struct {
	Conf                  *config.Config
	EthCli                *ethclient.Client
	Acc                   *types.Account
	ChainId               *big.Int
	MaxPriorityFeePerGas  *big.Int
	GasPrice              *big.Int
	ProcessedTransactions []common.Hash
	BlockNumsIncludingTx  []uint64
	AlreadyTestedRPCs     []*types.RpcResult
	ERC20Abi              *abi.ABI
	ERC20ByteCode         []byte
	ERC20Addr             common.Address
	FilterQuery           ethereum.FilterQuery
	FilterId              string
	BlockFilterId         string
}

func NewContext(conf *config.Config) (*RpcContext, error) {
	// Connect to the Ethereum client
	ethCli, err := ethclient.Dial(conf.RpcEndpoint)
	if err != nil {
		return nil, err
	}

	ecdsaPrivKey, err := crypto.HexToECDSA(conf.RichPrivKey)
	if err != nil {
		return nil, err
	}

	ctx := &RpcContext{
		Conf:   conf,
		EthCli: ethCli,
		Acc: &types.Account{
			Address: crypto.PubkeyToAddress(ecdsaPrivKey.PublicKey),
			PrivKey: ecdsaPrivKey,
		},
	}

	// Scan existing blockchain state to populate initial data
	err = ctx.loadExistingState()
	if err != nil {
		// Not a fatal error - we can continue with empty state
		fmt.Printf("Warning: Could not load existing blockchain state: %v\n", err)
	}

	return ctx, nil
}

// loadExistingState scans the blockchain and creates test transactions if needed
func (rCtx *RpcContext) loadExistingState() error {
	// First, scan existing blocks for any transactions
	blockNumber, err := rCtx.EthCli.BlockNumber(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get block number: %v", err)
	}

	// Scan recent blocks for any existing transactions
	startBlock := uint64(1) // Start from block 1 (genesis is 0)
	if blockNumber > 50 {
		startBlock = blockNumber - 50
	}

	fmt.Printf("Scanning blocks %d to %d for existing transactions...\n", startBlock, blockNumber)

	for i := startBlock; i <= blockNumber; i++ {
		block, err := rCtx.EthCli.BlockByNumber(context.Background(), big.NewInt(int64(i)))
		if err != nil {
			continue // Skip blocks we can't read
		}

		// Process transactions in this block
		for _, tx := range block.Transactions() {
			txHash := tx.Hash()
			
			// Get transaction receipt
			receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), txHash)
			if err != nil {
				continue // Skip transactions without receipts
			}

			// Add successful transactions to our list
			if receipt.Status == 1 {
				rCtx.ProcessedTransactions = append(rCtx.ProcessedTransactions, txHash)
				rCtx.BlockNumsIncludingTx = append(rCtx.BlockNumsIncludingTx, receipt.BlockNumber.Uint64())

				// If this transaction created a contract, save the address
				if receipt.ContractAddress != (common.Address{}) {
					rCtx.ERC20Addr = receipt.ContractAddress
					fmt.Printf("Found contract at address: %s (tx: %s)\n", receipt.ContractAddress.Hex(), txHash.Hex())
				}
			}
		}
	}

	fmt.Printf("Loaded %d existing transactions\n", len(rCtx.ProcessedTransactions))

	// If we don't have enough transactions, create some test transactions now
	if len(rCtx.ProcessedTransactions) < 3 {
		fmt.Printf("Note: Only %d transactions found. Consider running more transactions for comprehensive API testing.\n", len(rCtx.ProcessedTransactions))
		// TODO: Implement createTestTransactions method
	}

	if rCtx.ERC20Addr != (common.Address{}) {
		fmt.Printf("Contract available at: %s\n", rCtx.ERC20Addr.Hex())
	}

	return nil
}

func (rCtx *RpcContext) AlreadyTested(rpc types.RpcName) *types.RpcResult {
	for _, testedRPC := range rCtx.AlreadyTestedRPCs {
		if rpc == testedRPC.Method {
			return testedRPC
		}
	}
	return nil

}

func WaitForTx(rCtx *RpcContext, txHash common.Hash, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond) // Check every 500ms
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout exceeded while waiting for transaction %s", txHash.Hex())
		case <-ticker.C:
			receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), txHash)
			if err != nil && !errors.Is(err, ethereum.NotFound) {
				return err
			}
			if err == nil {
				rCtx.ProcessedTransactions = append(rCtx.ProcessedTransactions, txHash)
				rCtx.BlockNumsIncludingTx = append(rCtx.BlockNumsIncludingTx, receipt.BlockNumber.Uint64())
				rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, &types.RpcResult{
					Method: MethodNameEthGetTransactionReceipt,
					Status: types.Ok,
					Value:  utils.MustBeautifyReceipt(receipt),
				})
				if receipt.ContractAddress != (common.Address{}) {
					rCtx.ERC20Addr = receipt.ContractAddress
				}
				if receipt.Status == 0 {
					return fmt.Errorf("transaction %s failed", txHash.Hex())
				}
				return nil
			}
		}
	}
}

func Skipped(methodName types.RpcName, category string, reason string) (*types.RpcResult, error) {
	return &types.RpcResult{
		Method:   methodName,
		Status:   types.Skipped,
		ErrMsg:   reason,
		Category: category,
	}, nil
}

func Legacy(rCtx *RpcContext, methodName types.RpcName, category string, replacementInfo string) (*types.RpcResult, error) {
	// First test if the API is actually implemented
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, string(methodName))
	
	if err != nil {
		// Check if it's a "method not found" error (API not implemented)
		if err.Error() == "the method "+string(methodName)+" does not exist/is not available" ||
			err.Error() == "Method not found" ||
			err.Error() == string(methodName)+" method not found" {
			// API is not implemented, so it should be NOT_IMPL, not LEGACY
			return &types.RpcResult{
				Method:   methodName,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: category,
			}, nil
		}
		// API exists but failed with parameters (could be legacy with wrong params)
		// Still mark as legacy since the method exists
	}

	// API exists (either succeeded or failed with parameter issues), mark as LEGACY
	return &types.RpcResult{
		Method:   methodName,
		Status:   types.Legacy,
		Value:    fmt.Sprintf("Legacy API implemented in Cosmos EVM. %s", replacementInfo),
		ErrMsg:   replacementInfo,
		Category: category,
	}, nil
}

// Missing function implementations
func EthCoinbase(rCtx *RpcContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "eth_coinbase")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthCoinbase,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "eth",
		}, nil
	}

	return &types.RpcResult{
		Method:   MethodNameEthCoinbase,
		Status:   types.Ok,
		Value:    result,
		Category: "eth",
	}, nil
}

// Generic test handler that makes an actual RPC call to determine if an API is implemented
func GenericTest(rCtx *RpcContext, methodName types.RpcName, category string) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, string(methodName))

	if err != nil {
		// Check if it's a "method not found" error (API not implemented)
		if err.Error() == "the method "+string(methodName)+" does not exist/is not available" ||
			err.Error() == "Method not found" ||
			err.Error() == string(methodName)+" method not found" {
			return &types.RpcResult{
				Method:   methodName,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: category,
			}, nil
		}
		// Other errors mean the method exists but failed (could be parameter issues, etc.)
		return &types.RpcResult{
			Method:   methodName,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: category,
		}, nil
	}

	// Method exists and returned a result
	return &types.RpcResult{
		Method:   methodName,
		Status:   types.Ok,
		Value:    result,
		Category: category,
	}, nil
}

// MustLoadContractInfo loads contract information into the RPC context
func MustLoadContractInfo(rCtx *RpcContext) *RpcContext {
	// Read the ABI file
	abiFile, err := os.ReadFile("contracts/ERC20Token.abi")
	if err != nil {
		log.Fatalf("Failed to read ABI file: %v", err)
	}
	// Parse the ABI
	parsedABI, err := abi.JSON(strings.NewReader(string(abiFile)))
	if err != nil {
		log.Fatalf("Failed to parse ERC20 ABI: %v", err)
	}
	rCtx.ERC20Abi = &parsedABI
	// Read the compiled contract bytecode
	contractBytecode := common.FromHex(hex.EncodeToString(contracts.ContractByteCode))
	rCtx.ERC20ByteCode = contractBytecode

	// Load deployed contract addresses from registry
	evmdContract, _, err := utils.GetContractAddresses()
	if err != nil {
		log.Printf("Warning: Could not load contract addresses from registry: %v", err)
		log.Printf("Run 'go run main.go setup' first to deploy contracts")
	} else {
		// Use evmd contract address (since we're testing against evmd endpoint)
		rCtx.ERC20Addr = evmdContract
		log.Printf("Loaded contract address from registry: %s", rCtx.ERC20Addr.Hex())

		// Try to run a quick transaction generation to populate transaction data
		log.Println("Generating fresh test transactions for comprehensive API testing...")
		if err := generateTestTransactionsForRPC(rCtx); err != nil {
			log.Printf("Warning: Could not generate test transactions: %v", err)
			log.Printf("Some transaction-dependent API tests may fail")
		}
	}

	return rCtx
}

// generateTestTransactionsForRPC creates some test transactions to populate RPC context data
func generateTestTransactionsForRPC(rCtx *RpcContext) error {
	// Generate a few quick transactions using the transaction generation system
	evmdURL := "http://localhost:8545"

	// Create a few transaction scenarios specifically for RPC testing
	scenarios := []*utils.TransactionScenario{
		{
			Name:        "rpc_test_eth_transfer",
			Description: "ETH transfer for RPC testing",
			TxType:      "transfer",
			FromKey:     utils.Dev1PrivateKey,
			To:          &common.Address{0x01},        // Simple test address
			Value:       big.NewInt(1000000000000000), // 0.001 ETH
			GasLimit:    21000,
			ExpectFail:  false,
		},
	}

	// Connect to evmd
	evmdClient, err := ethclient.Dial(evmdURL)
	if err != nil {
		return fmt.Errorf("failed to connect to evmd: %w", err)
	}

	// Execute scenarios to generate transaction hashes
	for _, scenario := range scenarios {
		result, err := utils.ExecuteTransactionScenario(evmdClient, scenario, "evmd")
		if err != nil {
			log.Printf("Warning: Failed to execute test transaction %s: %v", scenario.Name, err)
			continue
		}

		if result.Success {
			// Add transaction hash to RPC context
			rCtx.ProcessedTransactions = append(rCtx.ProcessedTransactions, result.TxHash)
			if result.Receipt != nil {
				rCtx.BlockNumsIncludingTx = append(rCtx.BlockNumsIncludingTx, result.Receipt.BlockNumber.Uint64())
			}
			log.Printf("Generated test transaction: %s", result.TxHash.Hex())
		}
	}

	log.Printf("Generated %d test transactions for RPC testing", len(rCtx.ProcessedTransactions))
	return nil
}
