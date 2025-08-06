package utils

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ExecuteTransactionBatch executes a batch of transaction scenarios on both networks
func ExecuteTransactionBatch(evmdURL, gethURL string, evmdContract, gethContract common.Address) (*TransactionBatch, error) {
	batch := &TransactionBatch{
		Name:         "JSON-RPC Compatibility Test Batch",
		EvmdContract: evmdContract,
		GethContract: gethContract,
	}

	// Create transaction scenarios
	scenarios := CreateTransactionScenarios(evmdContract, gethContract)
	batch.Scenarios = scenarios

	// Connect to both networks
	evmdClient, err := ethclient.Dial(evmdURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to evmd: %w", err)
	}

	gethClient, err := ethclient.Dial(gethURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to geth: %w", err)
	}

	fmt.Printf("\n=== Executing Transaction Test Batch ===\n")
	fmt.Printf("Total scenarios: %d\n", len(scenarios))
	fmt.Printf("Networks: evmd (%s), geth (%s)\n", evmdURL, gethURL)
	fmt.Printf("Contracts: evmd=%s, geth=%s\n\n", evmdContract.Hex(), gethContract.Hex())

	// Execute scenarios on both networks
	for i, scenario := range scenarios {
		fmt.Printf("[%d/%d] Executing: %s\n", i+1, len(scenarios), scenario.Name)

		// Update contract address based on network
		evmdScenario := *scenario // Copy
		gethScenario := *scenario // Copy

		if scenario.TxType == "erc20_transfer" || scenario.TxType == "erc20_approve" {
			evmdScenario.To = &evmdContract
			gethScenario.To = &gethContract
		}

		// Execute on evmd
		fmt.Printf("  → evmd: ")
		evmdResult, err := ExecuteTransactionScenario(evmdClient, &evmdScenario, "evmd")
		if err != nil && !scenario.ExpectFail {
			fmt.Printf("ERROR - %s\n", err.Error())
		} else {
			if evmdResult.Success {
				fmt.Printf("✓ Success")
				if evmdResult.Receipt != nil {
					fmt.Printf(" (gas: %d, block: %s)", evmdResult.GasUsed, evmdResult.BlockNumber.String())
				}
				fmt.Printf("\n")
			} else {
				fmt.Printf("✗ Failed - %s\n", evmdResult.Error)
			}
		}
		batch.EvmdResults = append(batch.EvmdResults, evmdResult)

		// Small delay between networks to avoid nonce issues
		time.Sleep(100 * time.Millisecond)

		// Execute on geth
		fmt.Printf("  → geth: ")
		gethResult, err := ExecuteTransactionScenario(gethClient, &gethScenario, "geth")
		if err != nil && !scenario.ExpectFail {
			fmt.Printf("ERROR - %s\n", err.Error())
		} else {
			if gethResult.Success {
				fmt.Printf("✓ Success")
				if gethResult.Receipt != nil {
					fmt.Printf(" (gas: %d, block: %s)", gethResult.GasUsed, gethResult.BlockNumber.String())
				}
				fmt.Printf("\n")
			} else {
				fmt.Printf("✗ Failed - %s\n", gethResult.Error)
			}
		}
		batch.GethResults = append(batch.GethResults, gethResult)

		// Delay between scenarios
		time.Sleep(500 * time.Millisecond)
	}

	return batch, nil
}

// GenerateTransactionSummary creates a summary report of the transaction batch execution
func GenerateTransactionSummary(batch *TransactionBatch) string {
	if batch == nil {
		return "No transaction batch to summarize"
	}

	summary := "\n=== Transaction Batch Summary ===\n"
	summary += fmt.Sprintf("Batch: %s\n", batch.Name)
	summary += fmt.Sprintf("Total Scenarios: %d\n\n", len(batch.Scenarios))

	evmdSuccess := 0
	gethSuccess := 0

	for i, scenario := range batch.Scenarios {
		evmdResult := batch.EvmdResults[i]
		gethResult := batch.GethResults[i]

		summary += fmt.Sprintf("%d. %s (%s)\n", i+1, scenario.Name, scenario.Description)

		if evmdResult.Success {
			evmdSuccess++
			summary += "   evmd: ✓ Success"
			if evmdResult.Receipt != nil {
				summary += fmt.Sprintf(" | TX: %s | Gas: %d | Block: %s",
					evmdResult.TxHash.Hex()[:10]+"...",
					evmdResult.GasUsed,
					evmdResult.BlockNumber.String())
			}
			summary += "\n"
		} else {
			summary += fmt.Sprintf("   evmd: ✗ Failed - %s\n", evmdResult.Error)
		}

		if gethResult.Success {
			gethSuccess++
			summary += "   geth: ✓ Success"
			if gethResult.Receipt != nil {
				summary += fmt.Sprintf(" | TX: %s | Gas: %d | Block: %s",
					gethResult.TxHash.Hex()[:10]+"...",
					gethResult.GasUsed,
					gethResult.BlockNumber.String())
			}
			summary += "\n"
		} else {
			summary += fmt.Sprintf("   geth: ✗ Failed - %s\n", gethResult.Error)
		}

		summary += "\n"
	}

	summary += "=== Results Summary ===\n"
	summary += fmt.Sprintf("evmd: %d/%d successful (%.1f%%)\n",
		evmdSuccess, len(batch.Scenarios),
		float64(evmdSuccess)/float64(len(batch.Scenarios))*100)
	summary += fmt.Sprintf("geth: %d/%d successful (%.1f%%)\n",
		gethSuccess, len(batch.Scenarios),
		float64(gethSuccess)/float64(len(batch.Scenarios))*100)

	return summary
}

// GetTransactionHashes returns all transaction hashes from the batch for testing
func (batch *TransactionBatch) GetTransactionHashes() (evmdHashes, gethHashes []common.Hash) {
	for _, result := range batch.EvmdResults {
		if result.Success && result.TxHash != (common.Hash{}) {
			evmdHashes = append(evmdHashes, result.TxHash)
		}
	}

	for _, result := range batch.GethResults {
		if result.Success && result.TxHash != (common.Hash{}) {
			gethHashes = append(gethHashes, result.TxHash)
		}
	}

	return evmdHashes, gethHashes
}

// GetSuccessfulTransactions returns only successful transaction results
func (batch *TransactionBatch) GetSuccessfulTransactions() (evmdTxs, gethTxs []*TransactionResult) {
	for _, result := range batch.EvmdResults {
		if result.Success {
			evmdTxs = append(evmdTxs, result)
		}
	}

	for _, result := range batch.GethResults {
		if result.Success {
			gethTxs = append(gethTxs, result)
		}
	}

	return evmdTxs, gethTxs
}
