package main

import (
	"flag"
	"maps"
	"os"
	"slices"
	"strings"

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
		extTuplesFlag = flag.String("external-tuples", "", "External tuple mappings in format 'key1=value1,key2=value2'")
		imports       = flag.String("imports", "", "Additional import paths, comma-separated")
		stdlib        = flag.Bool("stdlib", false, "Generate stdlib itself")
		artifactInput = flag.Bool("artifact-input", false, "Input file is a solc artifact JSON, will extract the abi field from it")
	)
	flag.Parse()

	opts := []generator.Option{
		generator.PackageName(*packageName),
		generator.Prefix(*prefix),
		generator.Stdlib(*stdlib),
	}

	importSpecs := slices.Clone(DefaultExtraImports)
	if *imports != "" {
		paths := strings.Split(*imports, ",")
		for _, imp := range paths {
			importSpecs = append(importSpecs, generator.ParseImport(imp))
		}
	}
	opts = append(opts, generator.ExtraImports(importSpecs))

	// Parse external tuples if provided
	extTuples := maps.Clone(ExternalTuples)
	if *extTuplesFlag != "" {
		for k, v := range generator.ParseExternalTuples(*extTuplesFlag) {
			extTuples[k] = v
		}
	}
	opts = append(opts, generator.ExternalTuples(extTuples))

	generator.Command(
		*inputFile,
		*varName,
		*artifactInput,
		*outputFile,
		opts...,
	)
}
