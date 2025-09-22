//go:build test
// +build test

package types

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/core/vm"
	geth "github.com/ethereum/go-ethereum/params"
)

// testChainConfig is the chain configuration used in the EVM to defined which
// opcodes are active based on Ethereum upgrades.
var testChainConfig *ChainConfig

// Apply applies the changes to the virtual machine configuration.
func (ec *EvmConfig) Apply() error {
	// If Apply method has been already used in the object, return
	// an error to avoid overriding configuration.
	if IsSealed() {
		return fmt.Errorf("error applying EvmConfig: already sealed and cannot be modified")
	}

	if err := setTestChainConfig(ec.chainConfig); err != nil {
		return err
	}

	if err := extendDefaultExtraEIPs(ec.extendedDefaultExtraEIPs); err != nil {
		return err
	}

	if err := vm.ExtendActivators(ec.extendedEIPs); err != nil {
		return err
	}

	// After applying modifications, the configurator is sealed. This way, it is not possible
	// to call the configure method twice.
	Seal()

	return nil
}

func (ec *EvmConfig) ResetTestConfig() {
	vm.ResetActivators()
	resetEVMCoinInfo()
	testChainConfig = nil
	sealed = false
}

func setTestChainConfig(cc *ChainConfig) error {
	if testChainConfig != nil {
		return errors.New("chainConfig already set. Cannot set again the chainConfig. Call the configurators ResetTestConfig method before configuring a new chain.")
	}

	// If no chain config is provided, create a default one for testing
	if cc == nil {
		config := DefaultChainConfig(0, EvmCoinInfo{
			DisplayDenom:     "test",
			Decimals:         EighteenDecimals,
			ExtendedDecimals: EighteenDecimals,
		})
		cc = config
	}

	if err := cc.Validate(); err != nil {
		return err
	}

	testChainConfig = cc
	return nil
}

// GetEthChainConfig returns the `chainConfig` used in the EVM (geth type).
func GetEthChainConfig() *geth.ChainConfig {
	return testChainConfig.EthereumConfig(nil)
}

// GetChainConfig returns the `chainConfig`.
func GetChainConfig() *ChainConfig {
	return testChainConfig
}
