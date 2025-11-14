package testdata

import (
	contractutils "github.com/cosmos/evm/contracts/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

//go:generate go run ../../cmd -input StakingCaller.json -artifact-input -package stakingcaller -output stakingcaller/abi.go -external-tuples Description=staking.Description,CommissionRates=staking.CommissionRates,Redelegation=staking.Redelegation,RedelegationEntry=staking.RedelegationEntry,RedelegationOutput=staking.RedelegationOutput,RedelegationResponse=staking.RedelegationResponse,Validator=staking.Validator,UnbondingDelegationOutput=staking.UnbondingDelegationOutput -imports staking=github.com/cosmos/evm/precompiles/staking

func LoadStakingCallerContract() (evmtypes.CompiledContract, error) {
	return contractutils.LoadContractFromJSONFile("StakingCaller.json")
}
