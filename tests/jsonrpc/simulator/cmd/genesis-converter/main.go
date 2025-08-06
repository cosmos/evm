package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/genesis"
)

func main() {
	var (
		inputFile  = flag.String("input", "exported_genesis.json", "Input cosmos genesis file")
		outputFile = flag.String("output", "geth_genesis.json", "Output geth genesis file")
		help       = flag.Bool("h", false, "Show help")
	)
	flag.Parse()

	if *help {
		fmt.Println("Genesis Converter - Convert cosmos genesis to geth genesis format")
		fmt.Println()
		fmt.Println("USAGE:")
		fmt.Println("  genesis-converter [OPTIONS]")
		fmt.Println()
		fmt.Println("OPTIONS:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("DESCRIPTION:")
		fmt.Println("  Converts cosmos EVM genesis format to geth-compatible genesis format.")
		fmt.Println("  Focuses on account balances and EVM state (contracts and storage).")
		fmt.Println()
		fmt.Println("EXAMPLES:")
		fmt.Println("  genesis-converter")
		fmt.Println("  genesis-converter -input exported_genesis.json -output geth_genesis.json")
		fmt.Println()
		return
	}

	if err := genesis.ConvertFile(*inputFile, *outputFile); err != nil {
		log.Fatalf("Failed to convert genesis: %v", err)
	}
}