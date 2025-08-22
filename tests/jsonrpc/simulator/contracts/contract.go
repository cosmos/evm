package contracts

import (
	_ "embed"
)

//go:embed ERC20Token.bin
var ContractByteCode []byte
