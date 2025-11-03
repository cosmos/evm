package testdata

import (
	contractutils "github.com/cosmos/evm/contracts/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

//go:generate go run ../../cmd -input StakingCaller.json -artifact-input -output stakingcaller.abi.go

func LoadStakingCallerContract() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("StakingCaller.json")
}
