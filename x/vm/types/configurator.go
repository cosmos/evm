//
// The config package provides a convenient way to modify x/evm params and values.
// Its primary purpose is to be used during application initialization.

package types

import (
	"fmt"
	"slices"

	"github.com/ethereum/go-ethereum/core/vm"
)

// EVMConfigurator allows to extend x/evm module configurations. The configurator modifies
// the EVM before starting the node. This means that all init genesis validations will be
// applied to each change.
type EVMConfigurator struct {
	sealed                   bool
	extendedEIPs             map[int]func(*vm.JumpTable)
	extendedDefaultExtraEIPs []int64
	evmCoinInfo              EvmCoinInfo
}

// NewEVMConfigurator returns a pointer to a new EVMConfigurator object.
func NewEVMConfigurator() *EVMConfigurator {
	return &EVMConfigurator{}
}

// WithExtendedEips allows to add to the go-ethereum activators map the provided
// EIP activators.
func (ec *EVMConfigurator) WithExtendedEips(extendedEIPs map[int]func(*vm.JumpTable)) *EVMConfigurator {
	ec.extendedEIPs = extendedEIPs
	return ec
}

// WithExtendedDefaultExtraEIPs update the x/evm DefaultExtraEIPs params
// by adding provided EIP numbers.
func (ec *EVMConfigurator) WithExtendedDefaultExtraEIPs(eips ...int64) *EVMConfigurator {
	ec.extendedDefaultExtraEIPs = eips
	return ec
}

// WithEVMCoinInfo allows to define the denom and decimals of the token used as the
// EVM token.
func (ec *EVMConfigurator) WithEVMCoinInfo(coinInfo EvmCoinInfo) *EVMConfigurator {
	ec.evmCoinInfo = coinInfo
	return ec
}

func extendDefaultExtraEIPs(extraEIPs []int64) error {
	for _, eip := range extraEIPs {
		if slices.Contains(DefaultExtraEIPs, eip) {
			return fmt.Errorf("error configuring EVMConfigurator: EIP %d is already present in the default list: %v", eip, DefaultExtraEIPs)
		}

		DefaultExtraEIPs = append(DefaultExtraEIPs, eip)
	}
	return nil
}
