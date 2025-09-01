//go:build !test
// +build !test

package config

import (
	evmconfig "github.com/cosmos/evm/config"
	cosmosevmserverconfig "github.com/cosmos/evm/server/config"
)

// EvmAppOptions allows to setup the global configuration
// for the Cosmos EVM chain.
func EvmAppOptions(chainID uint64) error {
	evmCoinInfo := *cosmosevmserverconfig.DefaultEvmCoinInfo()
	return evmconfig.EvmAppOptionsWithConfig(chainID, evmCoinInfo, cosmosEVMActivators)
}
