//go:build test
// +build test

package config

import (
	evmconfig "github.com/cosmos/evm/config"
	cosmosevmserverconfig "github.com/cosmos/evm/server/config"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// GetTestEvmCoinInfo returns appropriate EvmCoinInfo for testing based on chainID.
// This replaces the old hardcoded TestChainsCoinInfo map with a function that
// creates configurations on demand.
func GetTestEvmCoinInfo(chainID uint64) evmtypes.EvmCoinInfo {
	switch chainID {
	case EighteenDecimalsChainID:
		return evmtypes.EvmCoinInfo{
			Denom:         ExampleChainDenom,
			ExtendedDenom: ExampleChainDenom,
			DisplayDenom:  ExampleDisplayDenom,
			Decimals:      evmtypes.EighteenDecimals,
		}
	case SixDecimalsChainID:
		return evmtypes.EvmCoinInfo{
			Denom:         "utest",
			ExtendedDenom: "atest",
			DisplayDenom:  "test",
			Decimals:      evmtypes.SixDecimals,
		}
	case TwelveDecimalsChainID:
		return evmtypes.EvmCoinInfo{
			Denom:         "ptest2",
			ExtendedDenom: "atest2",
			DisplayDenom:  "test2",
			Decimals:      evmtypes.TwelveDecimals,
		}
	case TwoDecimalsChainID:
		return evmtypes.EvmCoinInfo{
			Denom:         "ctest3",
			ExtendedDenom: "atest3",
			DisplayDenom:  "test3",
			Decimals:      evmtypes.TwoDecimals,
		}
	case TestChainID1, TestChainID2:
		return evmtypes.EvmCoinInfo{
			Denom:         ExampleChainDenom,
			ExtendedDenom: ExampleChainDenom,
			DisplayDenom:  ExampleChainDenom,
			Decimals:      evmtypes.EighteenDecimals,
		}
	case EVMChainID:
		return evmtypes.EvmCoinInfo{
			Denom:         ExampleChainDenom,
			ExtendedDenom: ExampleChainDenom,
			DisplayDenom:  ExampleDisplayDenom,
			Decimals:      evmtypes.EighteenDecimals,
		}
	default:
		// Default fallback - return the default configuration converted to EvmCoinInfo
		return cosmosevmserverconfig.DefaultChainConfig().ToEvmCoinInfo()
	}
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
	// Use the test coin info function to get the appropriate configuration
	coinInfo := GetTestEvmCoinInfo(chainID)
	return cosmosevmserverconfig.ChainConfig{
		Denom:         coinInfo.Denom,
		ExtendedDenom: coinInfo.ExtendedDenom,
		DisplayDenom:  coinInfo.DisplayDenom,
		Decimals:      uint8(coinInfo.Decimals),
	}
}
