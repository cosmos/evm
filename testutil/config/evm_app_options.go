//go:build !test
// +build !test

package config

import (
	evmconfig "github.com/cosmos/evm/config"
)

// EvmAppOptions allows to setup the global configuration
// for the Cosmos EVM chain using configuration.
func EvmAppOptions(chainID uint64) error {
	evmCoinInfo := GetEvmCoinInfo(chainID)
	return evmconfig.EvmAppOptions(chainID, evmCoinInfo, cosmosEVMActivators)
}

// EvmAppOptionsWithReset allows to setup the global configuration
// for the Cosmos EVM chain using configuration and optionally reset test config.
func EvmAppOptionsWithReset(chainID uint64, reset bool) error {
	evmCoinInfo := GetEvmCoinInfo(chainID)
	return evmconfig.EvmAppOptionsWithReset(chainID, evmCoinInfo, cosmosEVMActivators, reset)
}
