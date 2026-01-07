package contracts

import (
	"strings"

	_ "embed"
)

//go:embed Counter.abi
var counterABI []byte

//go:embed Counter.bin
var counterBin []byte

func CounterABIJSON() string {
	return string(counterABI)
}

func CounterBinHex() string {
	// solc emits hex without 0x and usually with a trailing newline
	hex := strings.TrimSpace(string(counterBin))
	if strings.HasPrefix(hex, "0x") {
		return hex
	}
	return "0x" + hex
}
