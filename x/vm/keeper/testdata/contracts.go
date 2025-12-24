package testdata

import (
	contractutils "github.com/cosmos/evm/contracts/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func LoadERC20Contract() (evmtypes.CompiledContract, error) {
	return contractutils.LegacyLoadContractFromJSONFile("ERC20Contract.json")
}

func LoadMessageCallContract() (evmtypes.CompiledContract, error) {
	return contractutils.LegacyLoadContractFromJSONFile("MessageCallContract.json")
}

func LoadDelegationTargetContract() (evmtypes.CompiledContract, error) {
	return contractutils.LegacyLoadContractFromJSONFile("DelegationTarget.json")
}

func LoadMaliciousDeployerContract() (evmtypes.CompiledContract, error) {
	return contractutils.LegacyLoadContractFromJSONFile("MaliciousDeployer.json")
}
