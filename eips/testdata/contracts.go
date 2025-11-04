package testdata

import (
	contractutils "github.com/cosmos/evm/contracts/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

//go:generate go run ../../precompiles/cmd -input Counter.json -artifact-input -output counter.abi.go
//go:generate go run ../../precompiles/cmd -input CounterFactory.json -artifact-input -output counter_factory.abi.go

func LoadCounterContract() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("Counter.json")
}

func LoadCounterFactoryContract() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("CounterFactory.json")
}
