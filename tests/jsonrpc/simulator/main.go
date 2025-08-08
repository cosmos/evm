package main

import (
	_ "embed"
	"flag"
	"log"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/report"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/rpc"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/utils"
)

func main() {
	verbose := flag.Bool("v", false, "Enable verbose output")
	outputExcel := flag.Bool("xlsx", false, "Save output as xlsx")
	flag.Parse()

	// Handle subcommand
	args := flag.Args()
	if len(args) > 0 && args[0] == "setup" {
		log.Println("Running setup: funding geth accounts and deploying contracts...")
		err := utils.RunSetup()
		if err != nil {
			log.Fatalf("Setup failed: %v", err)
		}
		log.Println("✓ Setup completed successfully!")
		return
	}

	if len(args) > 0 && args[0] == "txgen" {
		log.Println("Running transaction generation on both networks...")
		err := utils.RunTransactionGeneration()
		if err != nil {
			log.Fatalf("Transaction generation failed: %v", err)
		}
		log.Println("✓ Transaction generation completed successfully!")
		return
	}

	// Load configuration from conf.yaml
	conf := config.MustLoadConfig("config.yaml")

	rCtx, err := rpc.NewContext(conf)
	if err != nil {
		log.Fatalf("Failed to create context: %v", err)
	}

	rCtx = rpc.MustLoadContractInfo(rCtx)

	// Execute all tests
	results := rpc.ExecuteAllTests(rCtx)

	// Generate report
	report.Results(results, *verbose, *outputExcel)
}

