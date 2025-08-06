package utils

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ExecuteTransactionBatch executes a batch of transaction scenarios on both networks with enhanced metadata
func ExecuteTransactionBatch(evmdURL, gethURL string, evmdContract, gethContract common.Address) (*TransactionMetadataBatch, error) {
	batch := NewTransactionMetadataBatch("JSON-RPC Compatibility Test Batch", nil, evmdContract, gethContract)

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

		// Execute on evmd with metadata
		fmt.Printf("  → evmd: ")
		evmdMeta, err := ExecuteTransactionScenarioWithMetadata(evmdClient, evmdURL, &evmdScenario, "evmd")
		if err != nil && !scenario.ExpectFail {
			fmt.Printf("ERROR - %s\n", err.Error())
		} else {
			// Enhance metadata with additional fields
			EnhanceTransactionMetadata(evmdMeta)
			
			if evmdMeta.Success {
				fmt.Printf("✓ Success")
				if evmdMeta.Receipt != nil {
					fmt.Printf(" (gas: %d, block: %s, latency: %v)", 
						evmdMeta.GasUsed, evmdMeta.BlockNumber.String(), evmdMeta.APICallLatency)
				}
				fmt.Printf("\n")
			} else {
				fmt.Printf("✗ Failed - %s\n", evmdMeta.Error)
			}
		}
		batch.EvmdResults = append(batch.EvmdResults, evmdMeta)

		// Small delay between networks to avoid nonce issues
		time.Sleep(100 * time.Millisecond)

		// Execute on geth with metadata
		fmt.Printf("  → geth: ")
		gethMeta, err := ExecuteTransactionScenarioWithMetadata(gethClient, gethURL, &gethScenario, "geth")
		if err != nil && !scenario.ExpectFail {
			fmt.Printf("ERROR - %s\n", err.Error())
		} else {
			// Enhance metadata with additional fields
			EnhanceTransactionMetadata(gethMeta)
			
			if gethMeta.Success {
				fmt.Printf("✓ Success")
				if gethMeta.Receipt != nil {
					fmt.Printf(" (gas: %d, block: %s, latency: %v)", 
						gethMeta.GasUsed, gethMeta.BlockNumber.String(), gethMeta.APICallLatency)
				}
				fmt.Printf("\n")
			} else {
				fmt.Printf("✗ Failed - %s\n", gethMeta.Error)
			}
		}
		batch.GethResults = append(batch.GethResults, gethMeta)
		

		// Delay between scenarios
		time.Sleep(500 * time.Millisecond)
	}

	// Finalize batch metadata
	batch.EndTime = time.Now()
	batch.TotalLatency = batch.EndTime.Sub(batch.StartTime)
	
	// Count successes and failures
	for _, result := range batch.EvmdResults {
		if result.Success {
			batch.SuccessCount++
		} else {
			batch.FailureCount++
		}
	}
	
	return batch, nil
}

// GenerateTransactionSummary creates a summary report of the transaction batch execution
func GenerateTransactionSummary(batch *TransactionMetadataBatch) string {
	if batch == nil {
		return "No transaction batch to summarize"
	}

	summary := "\n=== Transaction Batch Summary ===\n"
	summary += fmt.Sprintf("Batch: %s\n", batch.Name)
	summary += fmt.Sprintf("Total Scenarios: %d\n", len(batch.Scenarios))
	summary += fmt.Sprintf("Total Duration: %v\n\n", batch.TotalLatency)

	evmdSuccess := 0
	gethSuccess := 0

	for i, scenario := range batch.Scenarios {
		if i < len(batch.EvmdResults) && i < len(batch.GethResults) {
			evmdResult := batch.EvmdResults[i]
			gethResult := batch.GethResults[i]

			summary += fmt.Sprintf("%d. %s (%s)\n", i+1, scenario.Name, scenario.Description)

			if evmdResult.Success {
				evmdSuccess++
				summary += "   evmd: ✓ Success"
				if evmdResult.Receipt != nil {
					summary += fmt.Sprintf(" | TX: %s | Gas: %d | Block: %s | Latency: %v",
						evmdResult.TxHash.Hex()[:10]+"...",
						evmdResult.GasUsed,
						evmdResult.BlockNumber.String(),
						evmdResult.APICallLatency)
				}
				summary += "\n"
			} else {
				summary += fmt.Sprintf("   evmd: ✗ Failed - %s\n", evmdResult.Error)
			}

			if gethResult.Success {
				gethSuccess++
				summary += "   geth: ✓ Success"
				if gethResult.Receipt != nil {
					summary += fmt.Sprintf(" | TX: %s | Gas: %d | Block: %s | Latency: %v",
						gethResult.TxHash.Hex()[:10]+"...",
						gethResult.GasUsed,
						gethResult.BlockNumber.String(),
						gethResult.APICallLatency)
				}
				summary += "\n"
			} else {
				summary += fmt.Sprintf("   geth: ✗ Failed - %s\n", gethResult.Error)
			}

			// Add basic comparison info if both succeeded
			if evmdResult.Success && gethResult.Success {
				gasMatch := evmdResult.GasUsed == gethResult.GasUsed
				latencyDiff := gethResult.APICallLatency - evmdResult.APICallLatency
				summary += fmt.Sprintf("   comparison: gasUsed_match=%v, latency_diff=%v\n",
					gasMatch, latencyDiff)
			}

			summary += "\n"
		}
	}

	summary += "=== Results Summary ===\n"
	summary += fmt.Sprintf("evmd: %d/%d successful (%.1f%%)\n",
		evmdSuccess, len(batch.Scenarios),
		float64(evmdSuccess)/float64(len(batch.Scenarios))*100)
	summary += fmt.Sprintf("geth: %d/%d successful (%.1f%%)\n",
		gethSuccess, len(batch.Scenarios),
		float64(gethSuccess)/float64(len(batch.Scenarios))*100)
	summary += fmt.Sprintf("Total batch success: %d, failures: %d\n", batch.SuccessCount, batch.FailureCount)

	return summary
}

// GetTransactionHashes returns all transaction hashes from the batch for testing
func (batch *TransactionMetadataBatch) GetTransactionHashes() (evmdHashes, gethHashes []common.Hash) {
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

// GetSuccessfulTransactions returns only successful transaction metadata
func (batch *TransactionMetadataBatch) GetSuccessfulTransactions() (evmdTxs, gethTxs []*TransactionMetadata) {
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
