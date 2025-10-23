package testdata

import (
	contractutils "github.com/cosmos/evm/contracts/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

//go:generate go run github.com/yihuang/go-abi/cmd -var ERC20MinterABI -output erc20minter.abi.go

var ERC20MinterABI = []string{
	"function mint(address to, uint256 amount)",
}

func LoadERC20NoMetadataContract() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("ERC20NoMetadata.json")
}
