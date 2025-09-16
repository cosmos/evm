package config

import (
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

type ChainConfig struct {
	ChainID   string
	EvmConfig *evmtypes.EvmConfig
}
