//go:build test
// +build test

package config

// EvmAppOptions allows to setup the global configuration
// for the Cosmos EVM chain.
func EvmAppOptions(chainID uint64) error {
	return EvmAppOptionsWithConfigWithReset(chainID, ChainsCoinInfo, cosmosEVMActivators, true)
}
