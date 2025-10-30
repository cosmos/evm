package types

import (
	"errors"

	gethparams "github.com/ethereum/go-ethereum/params"
)

// RuntimeConfig keeps the runtime configuration for the EVM module.
type RuntimeConfig struct {
	chainConfig    *ChainConfig
	ethChainConfig *gethparams.ChainConfig
	coinInfo       EvmCoinInfo
	extraEIPs      []int64
}

// NewRuntimeConfig builds a new runtime configuration from the provided values.
// Copies are made where necessary to avoid accidental mutation.
func NewRuntimeConfig(
	chainCfg *ChainConfig,
	ethCfg *gethparams.ChainConfig,
	coinInfo EvmCoinInfo,
	extraEIPs []int64,
) (*RuntimeConfig, error) {
	if chainCfg == nil {
		return nil, errors.New("runtime config requires chain config")
	}
	if ethCfg == nil {
		return nil, errors.New("runtime config requires eth chain config")
	}

	cfg := &RuntimeConfig{
		chainConfig:    chainCfg,
		ethChainConfig: ethCfg,
		coinInfo:       coinInfo,
	}

	if len(extraEIPs) > 0 {
		copied := make([]int64, len(extraEIPs))
		copy(copied, extraEIPs)
		cfg.extraEIPs = copied
	}

	return cfg, nil
}

// ChainConfig returns the x/vm chain configuration.
func (rc *RuntimeConfig) ChainConfig() *ChainConfig {
	return rc.chainConfig
}

// EthChainConfig returns the go-ethereum chain configuration.
func (rc *RuntimeConfig) EthChainConfig() *gethparams.ChainConfig {
	return rc.ethChainConfig
}

// CoinInfo returns the EVM coin info.
func (rc *RuntimeConfig) EvmCoinInfo() EvmCoinInfo {
	return rc.coinInfo
}

// ExtraEIPs returns the additional EIPs configured for the EVM.
func (rc *RuntimeConfig) ExtraEIPs() []int64 {
	if len(rc.extraEIPs) == 0 {
		return nil
	}
	copied := make([]int64, len(rc.extraEIPs))
	copy(copied, rc.extraEIPs)
	return copied
}
