package main

import (
	_ "embed"
	"flag"
	"log"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/report"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/runner"
)

func main() {
	verbose := flag.Bool("v", false, "Enable verbose output")
	outputExcel := flag.Bool("xlsx", false, "Save output as xlsx")
	flag.Parse()

	rCtx, err := runner.Setup()
	if err != nil {
		log.Fatalf("Setup failed: %v", err)
	}

	// Execute all tests
	results := runner.ExecuteAllTests(rCtx)

	// Generate report
	report.Results(results, *verbose, *outputExcel, rCtx)
}
