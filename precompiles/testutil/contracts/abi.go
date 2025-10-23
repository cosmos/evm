package contracts

//go:generate go run github.com/yihuang/go-abi/cmd -input ICS20Caller.json -artifact-input -output ics20caller/abi.go -external-tuples Coin=cmn.Coin -imports cmn=github.com/cosmos/evm/precompiles/common
//go:generate go run github.com/yihuang/go-abi/cmd -input DistributionCaller.json -artifact-input -package distcaller -output distcaller/abi.go -external-tuples Coin=cmn.Coin -imports cmn=github.com/cosmos/evm/precompiles/common
//go:generate go run github.com/yihuang/go-abi/cmd -input Counter.json -artifact-input -package counter -output counter/abi.go -external-tuples Coin=cmn.Coin -imports cmn=github.com/cosmos/evm/precompiles/common
//go:generate go run github.com/yihuang/go-abi/cmd -input FlashLoan.json -artifact-input -package flashloan -output flashloan/abi.go -external-tuples Coin=cmn.Coin -imports cmn=github.com/cosmos/evm/precompiles/common
//go:generate go run github.com/yihuang/go-abi/cmd -input GovCaller.json -artifact-input -package govcaller -output govcaller/abi.go -external-tuples Coin=cmn.Coin -imports cmn=github.com/cosmos/evm/precompiles/common
