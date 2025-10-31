//
// The config package provides a convenient way to modify x/evm params and values.
// Its primary purpose is to be used during application initialization.

//go:build !test
// +build !test

package types

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/vm"
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
