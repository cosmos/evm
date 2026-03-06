package contracts

import (
	contractutils "github.com/cosmos/evm/contracts/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func LoadICS20TransferTester() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("solidity/ICS20TransferTester.json")
}
