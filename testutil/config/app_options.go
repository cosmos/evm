package config

import (
	evmconfig "github.com/cosmos/evm/config"
)

// EvmAppOptions allows to setup the global configuration
// for the Cosmos EVM chain.
func EvmAppOptions(chainID uint64) error {
	evmCoinInfo := GetEvmCoinInfo(chainID)
	return evmconfig.EvmAppOptions(chainID, evmCoinInfo, cosmosEVMActivators)
}

// EvmAppOptionsWithReset allows to setup the global configuration
// for the Cosmos EVM chain using configuration with an optional reset.
func EvmAppOptionsWithReset(chainID uint64, reset bool) error {
	evmCoinInfo := GetEvmCoinInfo(chainID)
	return evmconfig.EvmAppOptionsWithReset(chainID, evmCoinInfo, cosmosEVMActivators, reset)
}
