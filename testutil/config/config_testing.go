//go:build test
// +build test

package config

import (
	evmconfig "github.com/cosmos/evm/config"
	cosmosevmserverconfig "github.com/cosmos/evm/server/config"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// GetTestEvmCoinInfo returns appropriate EvmCoinInfo for testing based on chainID.
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
		// Default fallback - return the default configuration
		return *cosmosevmserverconfig.DefaultEvmCoinInfo()
	}
}

// EvmAppOptions allows to setup the global configuration
// for the Cosmos EVM chain.
func EvmAppOptions(chainID uint64) error {
	evmCoinInfo := GetTestEvmCoinInfo(chainID)
	return evmconfig.EvmAppOptions(chainID, evmCoinInfo, cosmosEVMActivators)
}

// EvmAppOptionsWithReset allows to setup the global configuration
// for the Cosmos EVM chain using configuration with an optional reset.
func EvmAppOptionsWithReset(chainID uint64, withReset bool) error {
	evmCoinInfo := GetTestEvmCoinInfo(chainID)
	return evmconfig.EvmAppOptionsWithReset(chainID, evmCoinInfo, cosmosEVMActivators, withReset)
}
