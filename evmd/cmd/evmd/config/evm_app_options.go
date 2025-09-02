package config

import (
	evmconfig "github.com/cosmos/evm/config"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// EvmAppOptions allows to setup the global configuration
// for the Cosmos EVM chain using the provided chain configuration.
func EvmAppOptions(chainConfig ChainConfig) error {
	return evmconfig.EvmAppOptionsWithConfig(chainConfig.ChainInfo.EVMChainID, chainConfig.CoinInfo, cosmosEVMActivators)
}

// EvmAppOptionsFromConfig creates an EVM options function that uses the provided chain configuration.
func EvmAppOptionsFromConfig(chainConfig ChainConfig) evmconfig.EVMOptionsFn {
	return func(chainID uint64) error {
		// Use the chainID from the config, not the parameter (for consistency)
		return evmconfig.EvmAppOptionsWithConfig(chainConfig.ChainInfo.EVMChainID, chainConfig.CoinInfo, cosmosEVMActivators)
	}
}

// LegacyEvmAppOptions provides backward compatibility with the old interface.
// Deprecated: Use EvmAppOptionsFromConfig instead.
func LegacyEvmAppOptions(chainID uint64, coinInfo evmtypes.EvmCoinInfo) error {
	return evmconfig.EvmAppOptionsWithConfig(chainID, coinInfo, cosmosEVMActivators)
}
