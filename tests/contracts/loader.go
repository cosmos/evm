package contracts

import (
	contractutils "github.com/cosmos/evm/contracts/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

//go:generate go run ../../precompiles/cmd -input "account_abstraction/smartwallet/SimpleSmartWallet.json" -artifact-input -output smartwallet.abi.go
//go:generate go run ../../precompiles/cmd -input "account_abstraction//entrypoint/SimpleEntryPoint.json" -artifact-input -output entrypoint.abi.go -external-tuples UserOperation=UserOperation

func LoadSimpleERC20() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("account_abstraction/tokens/SimpleERC20.json")
}

func LoadSimpleEntryPoint() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("account_abstraction//entrypoint/SimpleEntryPoint.json")
}

func LoadSimpleSmartWallet() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("account_abstraction/smartwallet/SimpleSmartWallet.json")
}
