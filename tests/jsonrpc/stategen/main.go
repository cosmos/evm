package main

import (
	"flag"
	"log"
)

const (
	defaultEndpoint = "http://localhost:8545"
)

func main() {
	var (
		endpoint = flag.String("endpoint", defaultEndpoint, "RPC endpoint URL")
		help     = flag.Bool("h", false, "Show help")
	)
	flag.Parse()

	if *help {
		printHelp()
		return
	}

	// Create state generator
	generator, err := NewStateGenerator(*endpoint)
	if err != nil {
		log.Fatalf("Failed to create state generator: %v", err)
	}

	// Create initial state
	if err := generator.CreateInitialState(); err != nil {
		log.Fatalf("Failed to create initial state: %v", err)
	}

	log.Println("Initial state generation completed successfully!")
}

func printHelp() {
	println("stategen - Blockchain Initial State Generator")
	println("")
	println("USAGE:")
	println("  stategen [OPTIONS]")
	println("")
	println("OPTIONS:")
	flag.PrintDefaults()
	println("")
	println("DESCRIPTION:")
	println("  Creates initial blockchain state for JSON-RPC compatibility testing by:")
	println("  1. Sending a value transfer transaction")
	println("  2. Deploying an ERC20 contract")
	println("  3. Executing an ERC20 transfer transaction")
	println("")
	println("EXAMPLES:")
	println("  stategen")
	println("  stategen -endpoint http://localhost:8545")
	println("")
}