package config

import (
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

type ChainConfig struct {
	ChainID   string
	EvmConfig *evmtypes.EvmConfig
}

// CreateChainConfig allows creating a test chain with custom parameters
// extendedDecimals is always 18-decimals (atto) denom for EVM chains
func CreateChainConfig(
	chainID string,
	evmChainID uint64,
	evmChainConfig *evmtypes.ChainConfig,
	displayDenom string,
	decimals evmtypes.Decimals,
	extendedDecimals evmtypes.Decimals,
) ChainConfig {
	coinInfo := evmtypes.EvmCoinInfo{
		DisplayDenom:     displayDenom,
		Decimals:         decimals,
		ExtendedDecimals: extendedDecimals,
	}

	if evmChainConfig == nil {
		evmChainConfig = evmtypes.DefaultChainConfig(evmChainID, coinInfo)
	}
	evmConfig := evmtypes.NewEvmConfig().
		WithChainConfig(evmChainConfig).
		WithEVMCoinInfo(&coinInfo)

	return ChainConfig{
		ChainID:   chainID,
		EvmConfig: evmConfig,
	}
}
