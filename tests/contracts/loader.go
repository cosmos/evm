package contracts

import (
	contractutils "github.com/cosmos/evm/contracts/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func LoadSimpleERC20() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("SimpleERC20.json")
}

func LoadSimpleEntryPoint() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("SimpleEntryPoint.json")
}

func LoadSimpleSmartWallet() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("SimpleSmartWallet.json")
}
