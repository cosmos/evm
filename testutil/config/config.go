package config

import (
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

type ChainConfig struct {
	ChainInfo ChainInfo
	CoinInfo  evmtypes.EvmCoinInfo
}

type ChainInfo struct {
	ChainID    string
	EVMChainID uint64
}

// CreateChainConfig allows creating a test chain with custom parameters
func CreateChainConfig(chainID string, evmChainID uint64, displayDenom string, decimals evmtypes.Decimals) ChainConfig {
	// extended denom is always 18-decimals (atto) denom
	denom := evmtypes.CreateDenomStr(decimals, displayDenom)
	extendedDenom := evmtypes.CreateDenomStr(evmtypes.EighteenDecimals, displayDenom)

	return ChainConfig{
		ChainInfo: ChainInfo{
			ChainID:    chainID,
			EVMChainID: evmChainID,
		},
		CoinInfo: evmtypes.EvmCoinInfo{
			Denom:         denom,
			ExtendedDenom: extendedDenom,
			DisplayDenom:  displayDenom,
			Decimals:      decimals,
		},
	}
}
