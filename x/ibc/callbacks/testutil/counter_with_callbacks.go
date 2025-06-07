package testutil

import (
	contractutils "github.com/cosmos/evm/contracts/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func LoadCounterWithCallbacksContract() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("CounterWithCallbacks.json")
}
