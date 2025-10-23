package testdata

import (
	contractutils "github.com/cosmos/evm/contracts/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

//go:generate go run github.com/yihuang/go-abi/cmd -input ERC20TestCaller.json -artifact-input -output erc20caller.abi.go -external-tuples Coin=cmn.Coin -imports cmn=github.com/cosmos/evm/precompiles/common

func LoadERC20TestCaller() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("ERC20TestCaller.json")
}
