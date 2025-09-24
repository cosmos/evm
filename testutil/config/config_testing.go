//go:build test
// +build test

package config

import (
	"github.com/cosmos/evm/config"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// TestChainsCoinInfo is a map of the chain id and its corresponding EvmCoinInfo
// that allows initializing the app with different coin info based on the
// chain id
var TestChainsCoinInfo = map[uint64]evmtypes.EvmCoinInfo{ // TODO:VLAD - Remove this
	config.EighteenDecimalsChainID: {
		Denom:         config.ExampleChainDenom,
		ExtendedDenom: config.ExampleChainDenom,
		DisplayDenom:  config.ExampleDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals,
	},
	config.SixDecimalsChainID: {
		Denom:         "utest",
		ExtendedDenom: "atest",
		DisplayDenom:  "test",
		Decimals:      evmtypes.SixDecimals,
	},
	config.TwelveDecimalsChainID: {
		Denom:         "ptest2",
		ExtendedDenom: "atest2",
		DisplayDenom:  "test2",
		Decimals:      evmtypes.TwelveDecimals,
	},
	config.TwoDecimalsChainID: {
		Denom:         "ctest3",
		ExtendedDenom: "atest3",
		DisplayDenom:  "test3",
		Decimals:      evmtypes.TwoDecimals,
	},
	config.TestChainID1: {
		Denom:         config.ExampleChainDenom,
		ExtendedDenom: config.ExampleChainDenom,
		DisplayDenom:  config.ExampleChainDenom,
		Decimals:      evmtypes.EighteenDecimals,
	},
	config.TestChainID2: {
		Denom:         config.ExampleChainDenom,
		ExtendedDenom: config.ExampleChainDenom,
		DisplayDenom:  config.ExampleChainDenom,
		Decimals:      evmtypes.EighteenDecimals,
	},
	EVMChainID: {
		Denom:         config.ExampleChainDenom,
		ExtendedDenom: config.ExampleChainDenom,
		DisplayDenom:  config.ExampleDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals,
	},
}
