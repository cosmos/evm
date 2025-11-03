package main

import (
	"flag"
	"os"

	"github.com/yihuang/go-abi/generator"
)

var (
	// DefaultExtraImports adds common module to imports
	DefaultExtraImports = []generator.ImportSpec{
		{Path: "github.com/cosmos/evm/precompiles/common", Alias: "cmn"},
	}

	// DefaultExternalTuples mapps common tuples definitions to common module
	ExternalTuples = map[string]string{
		"Coin":         "cmn.Coin",
		"Dec":          "cmn.Dec",
		"DecCoin":      "cmn.DecCoin",
		"PageRequest":  "cmn.PageRequest",
		"PageResponse": "cmn.PageResponse",
		"Height":       "cmn.Height",
	}
)

func main() {
	var (
		inputFile     = flag.String("input", os.Getenv("GOFILE"), "Input file (JSON ABI or Go source file)")
		outputFile    = flag.String("output", "", "Output file")
		prefix        = flag.String("prefix", "", "Prefix for generated types and functions")
		packageName   = flag.String("package", os.Getenv("GOPACKAGE"), "Package name for generated code")
		varName       = flag.String("var", "", "Variable name containing human-readable ABI (for Go source files)")
		artifactInput = flag.Bool("artifact-input", false, "Input file is a solc artifact JSON, will extract the abi field from it")
	)
	flag.Parse()

	opts := []generator.Option{
		generator.PackageName(*packageName),
		generator.Prefix(*prefix),
		generator.ExtraImports(DefaultExtraImports),
		generator.ExternalTuples(ExternalTuples),
	}

	generator.Command(
		*inputFile,
		*varName,
		*artifactInput,
		*outputFile,
		opts...,
	)
}
