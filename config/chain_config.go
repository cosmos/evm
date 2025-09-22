package config

import (
	"github.com/cosmos/evm/config/eips"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func NewDefaultEvmConfig(
	evmChainID uint64,
	reset bool,
) *evmtypes.EvmConfig {
	chainConfig := evmtypes.DefaultChainConfig(evmtypes.DefaultEvmChainID)
	return evmtypes.NewEvmConfig().
		WithChainConfig(chainConfig).
		WithExtendedEips(eips.CosmosEVMActivators)
}
