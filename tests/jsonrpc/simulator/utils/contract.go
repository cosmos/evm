package utils

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/contracts"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

// MustLoadContractInfo loads contract information into the RPC context
func MustLoadContractInfo(rCtx *types.RPCContext) *types.RPCContext {
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
	evmdContract, _, err := GetContractAddresses()
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
func generateTestTransactionsForRPC(rCtx *types.RPCContext) error {
	// Generate a few quick transactions using the transaction generation system
	evmdURL := "http://localhost:8545"

	// Create a few transaction scenarios specifically for RPC testing
	scenarios := []*TransactionScenario{
		{
			Name:        "rpc_test_eth_transfer",
			Description: "ETH transfer for RPC testing",
			TxType:      "transfer",
			FromKey:     config.Dev1PrivateKey,
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
		result, err := ExecuteTransactionScenario(evmdClient, scenario, "evmd")
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
