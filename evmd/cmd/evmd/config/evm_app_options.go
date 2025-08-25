//go:build !test
// +build !test

package config

import (
	cosmosevmserverconfig "github.com/cosmos/evm/server/config"
)

// EvmAppOptions allows to setup the global configuration
// for the Cosmos EVM chain using dynamic configuration from app.toml.
func EvmAppOptions(chainID uint64) error {
	evmCoinInfo := *cosmosevmserverconfig.DefaultEvmCoinInfo()
	return EvmAppOptionsFromConfig(chainID, evmCoinInfo)
}
