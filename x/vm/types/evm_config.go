package types

import (
	"fmt"
	"slices"

	"github.com/ethereum/go-ethereum/core/vm"
)

// TOODO: remove singleton pattern and use a context instead
var sealed = false

func Seal() {
	sealed = true
}

func IsSealed() bool {
	return sealed
}

// EvmConfig allows to extend x/evm module configurations. The configurator modifies
// the EVM before starting the node. This means that all init genesis validations will be
// applied to each change.
type EvmConfig struct {
	extendedEIPs             map[int]func(*vm.JumpTable)
	extendedDefaultExtraEIPs []int64
	chainConfig              *ChainConfig
}

// NewEvmConfig returns a pointer to a new EvmConfig object.
func NewEvmConfig() *EvmConfig {
	return &EvmConfig{}
}

func (ec *EvmConfig) GetChainConfig() *ChainConfig {
	return ec.chainConfig
}

// WithChainConfig allows to define a custom `chainConfig` to be used in the
// EVM.
func (ec *EvmConfig) WithChainConfig(cc *ChainConfig) *EvmConfig {
	ec.chainConfig = cc
	return ec
}

// WithExtendedEips allows to add to the go-ethereum activators map the provided
// EIP activators.
func (ec *EvmConfig) WithExtendedEips(extendedEIPs map[int]func(*vm.JumpTable)) *EvmConfig {
	ec.extendedEIPs = extendedEIPs
	return ec
}

// WithExtendedDefaultExtraEIPs update the x/evm DefaultExtraEIPs params
// by adding provided EIP numbers.
func (ec *EvmConfig) WithExtendedDefaultExtraEIPs(eips ...int64) *EvmConfig {
	ec.extendedDefaultExtraEIPs = eips
	return ec
}

func extendDefaultExtraEIPs(extraEIPs []int64) error {
	for _, eip := range extraEIPs {
		if slices.Contains(DefaultExtraEIPs, eip) {
			return fmt.Errorf("error applying EvmConfig: EIP %d is already present in the default list: %v", eip, DefaultExtraEIPs)
		}

		DefaultExtraEIPs = append(DefaultExtraEIPs, eip)
	}
	return nil
}
