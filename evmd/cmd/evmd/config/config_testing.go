//go:build test
// +build test

package config

import (
	evmconfig "github.com/cosmos/evm/config"
	testconfig "github.com/cosmos/evm/testutil/config"
)

// EvmAppOptions allows to setup the global configuration
// for the Cosmos EVM chain using configuration.
func EvmAppOptions(chainID uint64) error {
	evmCoinInfo := testconfig.GetTestEvmCoinInfo(chainID)
	return evmconfig.EvmAppOptions(chainID, evmCoinInfo, cosmosEVMActivators)
}
