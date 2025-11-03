package testdata

import (
	contractutils "github.com/cosmos/evm/contracts/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

//go:generate go run ../../cmd -input StakingCallerTwo.json -artifact-input -package stakingcaller2 -output stakingcaller2/abi.go -external-tuples Description=staking.Description,CommissionRates=staking.CommissionRates -imports github.com/cosmos/evm/precompiles/staking

func LoadStakingCallerTwoContract() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("StakingCallerTwo.json")
}
