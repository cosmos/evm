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
		DisplayDenom:     ExampleDisplayDenom,
		Decimals:         evmtypes.EighteenDecimals,
		ExtendedDecimals: evmtypes.EighteenDecimals,
	},
	SixDecimalsChainID: {
		DisplayDenom:     "test",
		Decimals:         evmtypes.SixDecimals,
		ExtendedDecimals: evmtypes.EighteenDecimals,
	},
	TwelveDecimalsChainID: {
		DisplayDenom:     "test2",
		Decimals:         evmtypes.TwelveDecimals,
		ExtendedDecimals: evmtypes.EighteenDecimals,
	},
	TwoDecimalsChainID: {
		DisplayDenom:     "test3",
		Decimals:         evmtypes.TwoDecimals,
		ExtendedDecimals: evmtypes.EighteenDecimals,
	},
	TestChainID1: {
		DisplayDenom:     ExampleChainDenom,
		Decimals:         evmtypes.EighteenDecimals,
		ExtendedDecimals: evmtypes.EighteenDecimals,
	},
	TestChainID2: {
		DisplayDenom:     ExampleChainDenom,
		Decimals:         evmtypes.EighteenDecimals,
		ExtendedDecimals: evmtypes.EighteenDecimals,
	},
	EVMChainID: {
		DisplayDenom:     ExampleChainDenom,
		Decimals:         evmtypes.EighteenDecimals,
		ExtendedDecimals: evmtypes.EighteenDecimals,
	},
}

// EvmAppOptions allows to setup the global configuration
// for the Cosmos EVM chain.
func EvmAppOptions(chainID uint64) error {
	return evmconfig.EvmAppOptionsWithConfigWithReset(chainID, TestChainsCoinInfo, cosmosEVMActivators, true)
}
