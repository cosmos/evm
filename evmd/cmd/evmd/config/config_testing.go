//go:build test
// +build test

package config

import (
	evmconfig "github.com/cosmos/evm/config"
	cosmosevmserverconfig "github.com/cosmos/evm/server/config"
	testconfig "github.com/cosmos/evm/testutil/config"
)

// EvmAppOptions allows to setup the global configuration
// for the Cosmos EVM chain using dynamic configuration.
func EvmAppOptions(chainID uint64) error {
	// Get chain config from the test chain configs
	chainConfig := getTestChainConfigForChainID(chainID)
	evmCoinInfo := chainConfig.ToEvmCoinInfo()
	return evmconfig.EvmAppOptionsWithDynamicConfig(chainID, evmCoinInfo, cosmosEVMActivators)
}

// getTestChainConfigForChainID returns the appropriate chain config for testing
func getTestChainConfigForChainID(chainID uint64) cosmosevmserverconfig.ChainConfig {
	// Use the test coin info function to get the appropriate configuration
	coinInfo := testconfig.GetTestEvmCoinInfo(chainID)
	return cosmosevmserverconfig.ChainConfig{
		Denom:         coinInfo.Denom,
		ExtendedDenom: coinInfo.ExtendedDenom,
		DisplayDenom:  coinInfo.DisplayDenom,
		Decimals:      uint8(coinInfo.Decimals),
	}
}
