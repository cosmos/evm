//go:build !test
// +build !test

package config

import (
	evmconfig "github.com/cosmos/evm/config"
	cosmosevmserverconfig "github.com/cosmos/evm/server/config"
)

// EvmAppOptions allows to setup the global configuration
// for the Cosmos EVM chain using dynamic configuration.
func EvmAppOptions(chainID uint64) error {
	// Get chain config from the static chain configs for backward compatibility
	chainConfig := getChainConfigForChainID(chainID)
	evmCoinInfo := chainConfig.ToEvmCoinInfo()
	return evmconfig.EvmAppOptionsWithDynamicConfig(chainID, evmCoinInfo, cosmosEVMActivators)
}

// getChainConfigForChainID returns the appropriate chain config
func getChainConfigForChainID(chainID uint64) cosmosevmserverconfig.ChainConfig {
	// Use the coin info function to get the appropriate configuration
	coinInfo := GetEvmCoinInfo(chainID)
	return cosmosevmserverconfig.ChainConfig{
		Denom:         coinInfo.Denom,
		ExtendedDenom: coinInfo.ExtendedDenom,
		DisplayDenom:  coinInfo.DisplayDenom,
		Decimals:      uint8(coinInfo.Decimals),
	}
}
