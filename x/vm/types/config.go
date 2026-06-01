//
// The config package provides a convenient way to modify x/evm params and values.
// Its primary purpose is to be used during application initialization.

//go:build !test
// +build !test

package types

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/core/vm"
	geth "github.com/ethereum/go-ethereum/params"
)

// Configure applies the changes to the virtual machine configuration.
func (ec *EVMConfigurator) Configure() error {
	// If Configure method has been already used in the object, return
	// an error to avoid overriding configuration.
	if ec.sealed {
		return fmt.Errorf("error configuring EVMConfigurator: already sealed and cannot be modified")
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
	ec.sealed = true

	return nil
}

func (ec *EVMConfigurator) ResetTestConfig() {
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

// SetChainConfig allows to set the `chainConfig` variable modifying the
// default values. The method is private because it should only be called once
// in the EVMConfigurator.
func SetChainConfig(cc *ChainConfig) error {
	newConfig := DefaultChainConfig(0)
	if cc != nil {
		newConfig = cc
	}
	// Allow re-setting with the same chain ID (e.g. tempApp + real app in same process).
	if chainConfig != nil && chainConfig.ChainId != DefaultEVMChainID {
		if chainConfig.ChainId == newConfig.ChainId {
			return nil
		}
		return errors.New("chainConfig already set. Cannot set again the chainConfig")
	}
	if err := newConfig.Validate(); err != nil {
		return err
	}
	chainConfig = newConfig

	return nil
}
