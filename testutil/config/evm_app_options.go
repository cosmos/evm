//go:build !test
// +build !test

package config

import (
	evmconfig "github.com/cosmos/evm/config"
)

// EvmAppOptions allows to setup the global configuration
// for the Cosmos EVM chain.
func EvmAppOptions(chainID uint64) error {
	return evmconfig.EvmAppOptionsWithConfig(chainID, ChainsCoinInfo, cosmosEVMActivators)
}
