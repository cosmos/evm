package contracts

//go:generate go run ../../cmd -input "FlashLoan.json" -artifact-input -package flashloan -output flashloan/abi.go
//go:generate go run ../../cmd -input ICS20Caller.json -artifact-input -package ics20caller -output ics20caller/abi.go
//go:generate go run ../../cmd -input DistributionCaller.json -artifact-input -package distcaller -output distcaller/abi.go
//go:generate go run ../../cmd -input Counter.json -artifact-input -package counter -output counter/abi.go
//go:generate go run ../../cmd -input FlashLoan.json -artifact-input -package flashloan -output flashloan/abi.go
//go:generate go run ../../cmd -input GovCaller.json -artifact-input -package govcaller -output govcaller/abi.go
//go:generate go run ../../cmd -input StakingReverter.json -artifact-input -package stakingreverter -output stakingreverter/abi.go -external-tuples Description=staking.Description,CommissionRates=staking.CommissionRates,Redelegation=staking.Redelegation,RedelegationEntry=staking.RedelegationEntry,RedelegationOutput=staking.RedelegationOutput,RedelegationResponse=staking.RedelegationResponse,Validator=staking.Validator,UnbondingDelegationOutput=staking.UnbondingDelegationOutput -imports staking=github.com/cosmos/evm/precompiles/staking
//go:generate go run ../../cmd -input Reverter.json -artifact-input -package reverter -output reverter/abi.go
