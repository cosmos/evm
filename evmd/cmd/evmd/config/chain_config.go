package config

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

type ChainConfig struct {
	ChainID    string
	EvmChainID uint64
	EvmConfig  *types.EvmConfig
}

// NewChainConfig creates a chain config with custom parameters, using defaults if not provided
func NewChainConfig(
	chainID string,
	evmChainID uint64,
	evmActivators map[int]func(*vm.JumpTable),
	evmExtraEIPs []int64,
	evmChainConfig *types.ChainConfig,
	evmCoinInfo types.EvmCoinInfo,
	reset bool,
) ChainConfig {
	if evmChainConfig == nil {
		evmChainConfig = types.DefaultChainConfig(evmChainID, evmCoinInfo)
	}

	evmConfig := types.NewEvmConfig().
		WithChainConfig(evmChainConfig).
		WithEVMCoinInfo(evmCoinInfo).
		WithExtendedEips(evmActivators).
		WithExtendedDefaultExtraEIPs(evmExtraEIPs...)

	if reset {
		evmConfig.ResetTestConfig()
	}

	return ChainConfig{
		ChainID:    chainID,
		EvmChainID: evmChainID,
		EvmConfig:  evmConfig,
	}
}

// ApplyEvmConfig applies the evm config to the global singleton chain config and coin info
func (cc *ChainConfig) ApplyChainConfig() error {
	if cc.EvmConfig == nil {
		return nil // no op if evm config is nil
	}
	if cc.EvmConfig == nil {
		return fmt.Errorf("evm config is nil, cannot apply chain config")
	}
	if cc.EvmConfig.GetChainConfig() == nil {
		return fmt.Errorf("chain config is nil, cannot apply chain config")
	}

	if err := setBaseDenom(cc.EvmConfig.GetEVMCoinInfo()); err != nil {
		return err
	}
	return cc.EvmConfig.Apply()
}

// setBaseDenom registers the display denom and base denom and sets the
// base denom for the chain. The function registered different values based on
// the EvmCoinInfo to allow different configurations in mainnet and testnet.
func setBaseDenom(ci types.EvmCoinInfo) (err error) {
	// defer setting the base denom, and capture any potential error from it.
	// when failing because the denom was already registered, we ignore it and set
	// the corresponding denom to be base denom
	defer func() {
		err = sdk.SetBaseDenom(ci.GetDenom())
	}()
	if err := sdk.RegisterDenom(ci.DisplayDenom, math.LegacyOneDec()); err != nil {
		return err
	}

	// sdk.RegisterDenom will automatically overwrite the base denom when the
	// new setBaseDenom() units are lower than the current base denom's units.
	return sdk.RegisterDenom(ci.GetDenom(), math.LegacyNewDecWithPrec(1, int64(ci.Decimals)))
}
