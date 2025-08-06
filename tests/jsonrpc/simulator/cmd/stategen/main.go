package main

import (
	"flag"
	"log"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/stategen"
)

const defaultEndpoint = "http://localhost:8545"

func main() {
	var (
		endpoint = flag.String("endpoint", defaultEndpoint, "RPC endpoint URL")
		help     = flag.Bool("h", false, "Show help")
	)
	flag.Parse()

	if *help {
		log.Println("State Generator - Create initial blockchain state for testing")
		log.Println()
		log.Println("USAGE:")
		log.Println("  stategen [OPTIONS]")
		log.Println()
		log.Println("OPTIONS:")
		flag.PrintDefaults()
		log.Println()
		log.Println("DESCRIPTION:")
		log.Println("  Creates initial blockchain state by sending multiple transactions.")
		log.Println("  This includes value transfers to create accounts with balances.")
		log.Println()
		log.Println("EXAMPLES:")
		log.Println("  stategen")
		log.Println("  stategen -endpoint http://localhost:8545")
		log.Println()
		return
	}

	generator, err := stategen.NewStateGenerator(*endpoint)
	if err != nil {
		log.Fatalf("Failed to create state generator: %v", err)
	}

	if err := generator.CreateInitialState(); err != nil {
		log.Fatalf("Failed to create initial state: %v", err)
	}
}