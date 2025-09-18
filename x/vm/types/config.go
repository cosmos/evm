//go:build !test
// +build !test

package types

import (
	"github.com/ethereum/go-ethereum/core/vm"
	geth "github.com/ethereum/go-ethereum/params"
)

// Apply applies the changes to the virtual machine configuration.
func (ec *EvmConfig) Apply() error {
	// If Apply method has been already used in the object, return nil and no not overwrite
	// This mirrors the silent behavior of the previous EvmAppOptions implementation
	if IsSealed() {
		return nil
	}

	if err := setChainConfig(ec.chainConfig); err != nil {
		return err
	}

	if err := setEVMCoinInfo(ec.evmCoinInfo); err != nil {
		return err
	}

	if err := extendDefaultExtraEIPs(ec.extendedDefaultExtraEIPs); err != nil {
		return err
	}

	if err := vm.ExtendActivators(ec.extendedEIPs); err != nil {
		return err
	}

	// After applying modifiers the configurator is sealed. This way, it is not possible
	// to call the configure method twice.
	Seal()

	return nil
}

func (ec *EvmConfig) ResetTestConfig() {
	panic("this is only implemented with the 'test' build flag. Make sure you're running your tests using the '-tags=test' flag.")
}

// GetEthChainConfig returns the `chainConfig` used in the EVM (geth type).
func GetEthChainConfig() *geth.ChainConfig {
	return chainConfig.EthereumConfig(nil)
}

// GetChainConfig returns the `chainConfig`.
func GetChainConfig() *ChainConfig {
	return chainConfig
}
