// The config package provides a convenient way to modify x/evm params and values.
// Its primary purpose is to be used during application initialization.

package types

import (
	"github.com/ethereum/go-ethereum/core/vm"
)

// EvmConfig contains the specific EVM configuration details
type EvmConfig struct {
	ChainConfig  *ChainConfig
	CoinInfo     *EvmCoinInfo
	ExtendedEIPs map[int]func(*vm.JumpTable)
}

// NewEvmConfig returns a pointer to a new EvmConfig struct
func NewEvmConfig() *EvmConfig {
	return &EvmConfig{}
}

// WithChainConfig defines the custom EVM chain configuration
func (ec *EvmConfig) WithChainConfig(cc *ChainConfig) *EvmConfig {
	ec.ChainConfig = cc
	return ec
}

// WithEVMCoinInfo defines EVM coin info, including denom and decimals details
func (ec *EvmConfig) WithEVMCoinInfo(coinInfo *EvmCoinInfo) *EvmConfig {
	ec.CoinInfo = coinInfo
	return ec
}

// WithExtendedEips extends the global geth activators map with the provided EIP activators
func (ec *EvmConfig) WithExtendedEips(extendedEIPs map[int]func(*vm.JumpTable)) *EvmConfig {
	ec.ExtendedEIPs = extendedEIPs
	return ec
}

// ApplyExtendedEips adds the extended EIPs to the GLOBAL geth activators map
func (ec *EvmConfig) ApplyExtendedEips() error {
	return vm.ExtendActivators(ec.ExtendedEIPs)
}
