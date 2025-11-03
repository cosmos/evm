package keeper

import (
	"math/big"

	ethparams "github.com/ethereum/go-ethereum/params"

	"github.com/cosmos/evm/x/vm/types"
)

// ChainConfig returns the x/vm ChainConfig kept in runtime config.
func (k Keeper) ChainConfig() *types.ChainConfig {
	cfg := k.RuntimeConfig()
	if cfg == nil {
		return nil
	}
	return cfg.ChainConfig()
}

// EthChainConfig returns the go-ethereum ChainConfig kept in runtime config.
func (k Keeper) EthChainConfig() *ethparams.ChainConfig {
	cfg := k.RuntimeConfig()
	if cfg == nil {
		return nil
	}
	return cfg.EthChainConfig()
}

// EvmChainID returns the chain ID as a big.Int derived from the runtime config.
func (k Keeper) EvmChainID() *big.Int {
	chainCfg := k.ChainConfig()
	if chainCfg == nil {
		return big.NewInt(int64(types.DefaultEVMChainID)) //nolint:gosec // won't exceed int64
	}
	return big.NewInt(int64(chainCfg.ChainId)) //nolint:gosec // won't exceed int64
}

// effectiveEthChainConfig returns the runtime chain config if present, or falls back to the
// default configuration. This helper is intended for internal use.
func (k Keeper) effectiveEthChainConfig() *ethparams.ChainConfig {
	if cfg := k.EthChainConfig(); cfg != nil {
		return cfg
	}
	if chainCfg := k.ChainConfig(); chainCfg != nil {
		return chainCfg.EthereumConfig(nil)
	}
	return types.DefaultChainConfig(types.DefaultEVMChainID).EthereumConfig(nil)
}
