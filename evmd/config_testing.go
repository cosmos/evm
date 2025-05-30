//go:build test
// +build test

package evmd

import (
	"fmt"

	"github.com/cosmos/evm/cmd/evmd/config"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ChainsCoinInfo is a map of the chain id and its corresponding EvmCoinInfo
// that allows initializing the app with different coin info based on the
// chain id
var ChainsCoinInfo = map[uint64]evmtypes.EvmCoinInfo{
	config.EighteenDecimalsChainID: {
		Denom:         config.ExampleChainDenom,
		ExtendedDenom: config.ExampleChainDenom,
		DisplayDenom:  config.ExampleDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals,
	},
	config.SixDecimalsChainID: {
		Denom:         "utest",
		ExtendedDenom: "atest",
		DisplayDenom:  "test",
		Decimals:      evmtypes.SixDecimals,
	},
	config.TwelveDecimalsChainID: {
		Denom:         "ptest2",
		ExtendedDenom: "atest2",
		DisplayDenom:  "test2",
		Decimals:      evmtypes.TwelveDecimals,
	},
	config.TwoDecimalsChainID: {
		Denom:         "ctest3",
		ExtendedDenom: "atest3",
		DisplayDenom:  "test3",
		Decimals:      evmtypes.TwoDecimals,
	},
	config.TestChainID1: {
		Denom:         config.ExampleChainDenom,
		ExtendedDenom: config.ExampleChainDenom,
		DisplayDenom:  config.ExampleChainDenom,
		Decimals:      evmtypes.EighteenDecimals,
	},
	config.TestChainID2: {
		Denom:         config.ExampleChainDenom,
		ExtendedDenom: config.ExampleChainDenom,
		DisplayDenom:  config.ExampleChainDenom,
		Decimals:      evmtypes.EighteenDecimals,
	},
}

// EVMOptionsFn defines a function type for setting app options specifically for
// the Cosmos EVM app. The function should receive the chainID and return an error if
// any.
type EVMOptionsFn func(uint64) error

// NoOpEVMOptions is a no-op function that can be used when the app does not
// need any specific configuration.
func NoOpEVMOptions(_ uint64) error {
	return nil
}

// EvmAppOptions allows to setup the global configuration
// for the Cosmos EVM chain.
func EvmAppOptions(chainID uint64) error {
	coinInfo, found := ChainsCoinInfo[chainID]
	if !found {
		return fmt.Errorf("unknown chain id: %d", chainID)
	}

	// set the base denom considering if its mainnet or testnet
	if err := setBaseDenom(coinInfo); err != nil {
		return err
	}

	ethCfg := evmtypes.DefaultChainConfig(chainID)

	configurator := evmtypes.NewEVMConfigurator()
	// reset configuration to set the new one
	configurator.ResetTestConfig()
	err := configurator.
		WithExtendedEips(cosmosEVMActivators).
		WithChainConfig(ethCfg).
		WithEVMCoinInfo(coinInfo).
		Configure()
	if err != nil {
		return err
	}

	return nil
}

// setBaseDenom registers the display denom and base denom and sets the
// base denom for the chain. The function registered different values based on
// the EvmCoinInfo to allow different configurations in mainnet and testnet.
func setBaseDenom(ci evmtypes.EvmCoinInfo) (err error) {
	// Defer setting the base denom, and capture any potential error from it.
	// So when failing because the denom was already registered, we ignore it and set
	// the corresponding denom to be base denom
	defer func() {
		err = sdk.SetBaseDenom(ci.Denom)
	}()
	if err := sdk.RegisterDenom(ci.DisplayDenom, math.LegacyOneDec()); err != nil {
		return err
	}

	// sdk.RegisterDenom will automatically overwrite the base denom when the
	// new setBaseDenom() units are lower than the current base denom's units.
	return sdk.RegisterDenom(ci.Denom, math.LegacyNewDecWithPrec(1, int64(ci.Decimals)))
}
