package contracts

import (
	contractutils "github.com/cosmos/evm/contracts/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func LoadSequentialOperationsTester() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("solidity/SequentialOperationsTester.json")
}
