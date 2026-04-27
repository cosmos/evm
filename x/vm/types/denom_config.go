//
// The config package provides a convenient way to modify x/evm params and values.
// Its primary purpose is to be used during application initialization.

//go:build !test

package types

import (
	"fmt"
	"sync/atomic"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// evmCoinInfo hold the information of the coin used in the EVM as gas token. It
// can only be set via `EVMConfigurator` before starting the app.
var evmCoinInfo atomic.Pointer[EvmCoinInfo]

// GetCoinInfo returns EvmCoinInfo if set, otherwise panics.
func GetCoinInfo() EvmCoinInfo {
	return *getCoinInfo()
}

// GetEVMCoinDecimals returns the decimals used in the representation of the EVM coin.
func GetEVMCoinDecimals() Decimals {
	return Decimals(getCoinInfo().Decimals)
}

// GetEVMCoinDenom returns the denom used for the EVM coin.
func GetEVMCoinDenom() string {
	return getCoinInfo().Denom
}

// GetEVMCoinExtendedDenom returns the extended denom used for the EVM coin.
func GetEVMCoinExtendedDenom() string {
	return getCoinInfo().ExtendedDenom
}

// GetEVMCoinDisplayDenom returns the display denom used for the EVM coin.
func GetEVMCoinDisplayDenom() string {
	return getCoinInfo().DisplayDenom
}

// getCoinInfo return evmCoinInfo or panics if not present!
func getCoinInfo() *EvmCoinInfo {
	if info := evmCoinInfo.Load(); info != nil {
		return info
	}

	panic("global evmCoinInfo is not set yet!")
}

func setCoinInfo(info EvmCoinInfo) error {
	if err := validateCoinInfo(&info); err != nil {
		return err
	}

	evmCoinInfo.Store(&info)

	return nil
}

func validateCoinInfo(info *EvmCoinInfo) error {
	if err := sdk.ValidateDenom(info.Denom); err != nil {
		return fmt.Errorf("invalid EVM denom: %w", err)
	}

	if err := sdk.ValidateDenom(info.ExtendedDenom); err != nil {
		return fmt.Errorf("invalid EVM extended denom: %w", err)
	}

	if err := sdk.ValidateDenom(info.DisplayDenom); err != nil {
		return fmt.Errorf("invalid EVM display denom: %w", err)
	}

	if err := Decimals(info.Decimals).Validate(); err != nil {
		return fmt.Errorf("invalid EVM decimals: %w", err)
	}

	if Decimals(info.Decimals) == EighteenDecimals && info.Denom != info.ExtendedDenom {
		return fmt.Errorf("EVM denom and extended denom must be the same for 18 decimals")
	}

	return nil
}
