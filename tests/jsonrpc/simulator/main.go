package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/report"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/runner"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/utils"
)

func main() {
	verbose := flag.Bool("v", false, "Enable verbose output")
	outputExcel := flag.Bool("xlsx", false, "Save output as xlsx")
	flag.Parse()

	// Load configuration from conf.yaml
	conf := config.MustLoadConfig("config.yaml")

	// Create RPC context
	rCtx, err := types.NewRPCContext(conf)
	if err != nil {
		log.Fatalf("Failed to create context: %v", err)
	}

	err = utils.RunSetup(rCtx)
	if err != nil {
		log.Fatalf("Setup failed: %v", err)
	}

	err = utils.RunTransactionGeneration(rCtx)
	if err != nil {
		log.Fatalf("Transaction generation failed: %v", err)
	}

	// // Populate geth state for dual API comparison if enabled
	// if rCtx.EnableComparison {
	// 	log.Println("Populating geth state for dual API comparison...")
	// 	err := populateGethStateForComparison(rCtx)
	// 	if err != nil {
	// 		log.Printf("Warning: Failed to populate geth state: %v", err)
	// 		log.Println("Dual API comparison will be limited")
	// 	}
	// }

	// Execute all tests
	results := runner.ExecuteAllTests(rCtx)

	// Generate report
	report.Results(results, *verbose, *outputExcel, rCtx)
}

// populateGethStateForComparison populates geth state using ExecuteTransactionBatch
func populateGethStateForComparison(rCtx *types.RPCContext) error {
	// URLs for both networks
	evmdURL := rCtx.Conf.RpcEndpoint
	gethURL := "http://localhost:8547" // Default geth endpoint

	log.Println("Loading contract addresses for transaction batch...")

	// Get contract addresses - use existing evmd contract if available, deploy new ones if needed
	var evmdContract string
	if rCtx.EvmdCtx.ERC20Addr != (common.Address{}) {
		evmdContract = rCtx.EvmdCtx.ERC20Addr.Hex()
		log.Printf("Using existing evmd contract: %s", evmdContract)
	} else {
		log.Println("No existing evmd contract found, will deploy during batch execution")
		evmdContract = "" // Let ExecuteTransactionBatch handle deployment
	}

	// Convert contract address strings to common.Address
	var evmdAddr, gethAddr common.Address
	if evmdContract != "" {
		evmdAddr = common.HexToAddress(evmdContract)
	}
	// gethAddr will be zero address initially, ExecuteTransactionBatch will deploy

	log.Println("Executing transaction batch on both networks...")
	batch, err := utils.ExecuteTransactionBatch(evmdURL, gethURL, evmdAddr, gethAddr)
	if err != nil {
		return fmt.Errorf("failed to execute transaction batch: %w", err)
	}

	log.Println("Extracting geth transaction data from batch results...")

	// Extract geth data from batch results
	_, gethHashes := batch.GetTransactionHashes()
	var gethBlocks []uint64

	// Extract block numbers from successful geth transactions
	_, gethTxs := batch.GetSuccessfulTransactions()
	for _, tx := range gethTxs {
		if tx.BlockNumber != nil {
			gethBlocks = append(gethBlocks, tx.BlockNumber.Uint64())
		}
	}

	// Update the RPCContext with geth state data
	rCtx.UpdateGethStateFromBatch(gethHashes, batch.GethContract, gethBlocks)

	log.Printf("Successfully populated geth state: %d transactions, contract at %s",
		len(gethHashes), batch.GethContract.Hex())

	return nil
}
