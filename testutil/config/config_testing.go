//go:build test
// +build test

package config

import (
	evmconfig "github.com/cosmos/evm/config"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// TestChainsCoinInfo is a map of the chain id and its corresponding EvmCoinInfo
// that allows initializing the app with different coin info based on the
// chain id
var TestChainsCoinInfo = map[uint64]evmtypes.EvmCoinInfo{
	EighteenDecimalsChainID: {
		Denom:         ExampleChainDenom,
		ExtendedDenom: ExampleChainDenom,
		DisplayDenom:  ExampleDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals,
	},
	SixDecimalsChainID: {
		Denom:         "utest",
		ExtendedDenom: "atest",
		DisplayDenom:  "test",
		Decimals:      evmtypes.SixDecimals,
	},
	TwelveDecimalsChainID: {
		Denom:         "ptest2",
		ExtendedDenom: "atest2",
		DisplayDenom:  "test2",
		Decimals:      evmtypes.TwelveDecimals,
	},
	TwoDecimalsChainID: {
		Denom:         "ctest3",
		ExtendedDenom: "atest3",
		DisplayDenom:  "test3",
		Decimals:      evmtypes.TwoDecimals,
	},
	TestChainID1: {
		Denom:         ExampleChainDenom,
		ExtendedDenom: ExampleChainDenom,
		DisplayDenom:  ExampleChainDenom,
		Decimals:      evmtypes.EighteenDecimals,
	},
	TestChainID2: {
		Denom:         ExampleChainDenom,
		ExtendedDenom: ExampleChainDenom,
		DisplayDenom:  ExampleChainDenom,
		Decimals:      evmtypes.EighteenDecimals,
	},
	EVMChainID: {
		Denom:         ExampleChainDenom,
		ExtendedDenom: ExampleChainDenom,
		DisplayDenom:  ExampleDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals,
	},
}

// EvmAppOptions allows to setup the global configuration
// for the Cosmos EVM chain.
func EvmAppOptions(chainID uint64) error {
	return evmconfig.EvmAppOptionsWithConfigWithReset(chainID, TestChainsCoinInfo, cosmosEVMActivators, true)
}
