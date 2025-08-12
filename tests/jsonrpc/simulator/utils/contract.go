package utils

import (
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

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
			rCtx.EvmdCtx.ProcessedTransactions = append(rCtx.EvmdCtx.ProcessedTransactions, result.TxHash)
			if result.Receipt != nil {
				rCtx.EvmdCtx.BlockNumsIncludingTx = append(rCtx.EvmdCtx.BlockNumsIncludingTx, result.Receipt.BlockNumber.Uint64())
			}
			log.Printf("Generated test transaction: %s", result.TxHash.Hex())
		}
	}

	log.Printf("Generated %d test transactions for RPC testing", len(rCtx.EvmdCtx.ProcessedTransactions))

	// Connect to geth
	gethClient, err := ethclient.Dial("http://localhost:8547")
	if err != nil {
		return fmt.Errorf("failed to connect to evmd: %w", err)
	}

	// Execute scenarios to generate transaction hashes
	for _, scenario := range scenarios {
		result, err := ExecuteTransactionScenario(gethClient, scenario, "evmd")
		if err != nil {
			log.Printf("Warning: Failed to execute test transaction %s: %v", scenario.Name, err)
			continue
		}

		if result.Success {
			// Add transaction hash to RPC context
			rCtx.EvmdCtx.ProcessedTransactions = append(rCtx.EvmdCtx.ProcessedTransactions, result.TxHash)
			if result.Receipt != nil {
				rCtx.EvmdCtx.BlockNumsIncludingTx = append(rCtx.EvmdCtx.BlockNumsIncludingTx, result.Receipt.BlockNumber.Uint64())
			}
			log.Printf("Generated test transaction: %s", result.TxHash.Hex())
		}
	}

	return nil
}
