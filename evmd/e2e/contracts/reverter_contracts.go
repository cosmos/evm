package contracts

import (
	"strings"

	_ "embed"
)

//go:embed Reverter.abi
var reverterABI []byte

//go:embed Reverter.bin
var reverterBin []byte

func ReverterABIJSON() string {
	return string(reverterABI)
}

func ReverterBinHex() string {
	hex := strings.TrimSpace(string(reverterBin))
	if strings.HasPrefix(hex, "0x") {
		return hex
	}
	return "0x" + hex
}
