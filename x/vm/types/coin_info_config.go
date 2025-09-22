//go:build !test
// +build !test

package types

import (
	"errors"
	"sync"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// evmCoinInfo hold the information of the coin used in the EVM as gas token. It
// can only be set via `EvmConfig.Apply` before starting the app.
var (
	evmCoinInfo     *EvmCoinInfo
	evmCoinInfoOnce sync.Once
)

// GetEVMCoinDisplayDenom returns the display denom used for the EVM coin.
func GetEVMCoinDisplayDenom() string {
	return evmCoinInfo.DisplayDenom
}

// GetEVMCoinDecimals returns the decimals used in the representation of the EVM
// coin.
func GetEVMCoinDecimals() Decimals {
	return evmCoinInfo.Decimals
}

// GetEVMCoinExtendedDecimals returns the extended decimals used in the
// representation of the EVM coin.
func GetEVMCoinExtendedDecimals() Decimals {
	return 18
}

// GetEVMCoinDenom returns the denom used for the EVM coin.
func GetEVMCoinDenom() string {
	return evmCoinInfo.GetDenom()
}

// GetEVMCoinExtendedDenom returns the extended denom used for the EVM coin.
func GetEVMCoinExtendedDenom() string {
	return evmCoinInfo.GetExtendedDenom()
}

// SetEVMCoinInfo allows to define denom and decimals of the coin used in the EVM.
func SetEVMCoinInfo(eci EvmCoinInfo) error {
	if evmCoinInfo != nil {
		return errors.New("EVM coin info already set")
	}

	// prevent any external pointers or references to evmCoinInfoxx
	evmCoinInfoOnce.Do(func() {
		setBaseDenom(eci)
		evmCoinInfo = &eci
	})

	return nil
}

// setBaseDenom registers the display denom and base denom and sets the
// base denom for the chain. The function registered different values based on
// the EvmCoinInfo to allow different configurations in mainnet and testnet.
// TODO: look into deprecating this if it is not needed
func setBaseDenom(ci EvmCoinInfo) (err error) {
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
