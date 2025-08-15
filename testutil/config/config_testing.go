//go:build test
// +build test

package config

import (
	evmconfig "github.com/cosmos/evm/config"
	cosmosevmserverconfig "github.com/cosmos/evm/server/config"
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
// for the Cosmos EVM chain using dynamic configuration.
func EvmAppOptions(chainID uint64) error {
	// Get chain config from the test chain configs
	chainConfig := getTestChainConfigForChainID(chainID)
	evmCoinInfo := chainConfig.ToEvmCoinInfo()
	return evmconfig.EvmAppOptionsWithDynamicConfig(chainID, evmCoinInfo, cosmosEVMActivators)
}

// EvmAppOptionsWithReset allows to setup the global configuration
// for the Cosmos EVM chain using dynamic configuration with an optional reset.
func EvmAppOptionsWithReset(chainID uint64, withReset bool) error {
	// Get chain config from the test chain configs
	chainConfig := getTestChainConfigForChainID(chainID)
	evmCoinInfo := chainConfig.ToEvmCoinInfo()
	return evmconfig.EvmAppOptionsWithDynamicConfigWithReset(chainID, evmCoinInfo, cosmosEVMActivators, withReset)
}

// getTestChainConfigForChainID returns the appropriate chain config for testing
func getTestChainConfigForChainID(chainID uint64) cosmosevmserverconfig.ChainConfig {
	// Use the test chain coin info map to get the appropriate configuration
	if coinInfo, found := TestChainsCoinInfo[chainID]; found {
		return cosmosevmserverconfig.ChainConfig{
			Denom:         coinInfo.Denom,
			ExtendedDenom: coinInfo.ExtendedDenom,
			DisplayDenom:  coinInfo.DisplayDenom,
			Decimals:      uint8(coinInfo.Decimals),
		}
	}
	// Default fallback
	return *cosmosevmserverconfig.DefaultChainConfig()
}
