//
// The config package provides a convenient way to modify x/evm params and values.
// Its primary purpose is to be used during application initialization.

//go:build !test
// +build !test

package types

import (
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// evmCoinInfo hold the information of the coin used in the EVM as gas token. It
// can only be set via `EVMConfigurator` before starting the app.
var evmCoinInfo *EvmCoinInfo

// setEVMCoinDecimals allows to define the decimals used in the representation
// of the EVM coin.
func setEVMCoinDecimals(d Decimals) error {
	if err := d.Validate(); err != nil {
		return fmt.Errorf("setting EVM coin decimals: %w", err)
	}

	evmCoinInfo.Decimals = d
	return nil
}

// setEVMCoinDenom allows to define the denom of the coin used in the EVM.
func setEVMCoinDenom(denom string) error {
	if err := sdk.ValidateDenom(denom); err != nil {
		return fmt.Errorf("setting EVM coin denom: %w", err)
	}
	evmCoinInfo.Denom = denom
	return nil
}

// setEVMCoinExtendedDenom allows to define the extended denom of the coin used in the EVM.
func setEVMCoinExtendedDenom(extendedDenom string) error {
	if err := sdk.ValidateDenom(extendedDenom); err != nil {
		return err
	}
	evmCoinInfo.ExtendedDenom = extendedDenom
	return nil
}

// GetEVMCoinDecimals returns the decimals used in the representation of the EVM
// coin.
func GetEVMCoinDecimals() Decimals {
	return evmCoinInfo.Decimals
}

// GetEVMCoinDenom returns the denom used for the EVM coin.
func GetEVMCoinDenom() string {
	return evmCoinInfo.Denom
}

// GetEVMCoinExtendedDenom returns the extended denom used for the EVM coin.
func GetEVMCoinExtendedDenom() string {
	return evmCoinInfo.ExtendedDenom
}

// setEVMCoinInfo allows to define denom and decimals of the coin used in the EVM.
func setEVMCoinInfo(eci EvmCoinInfo) error {
	if evmCoinInfo != nil {
		return errors.New("EVM coin info already set")
	}

	if eci.Decimals == EighteenDecimals {
		if eci.Denom != eci.ExtendedDenom {
			return errors.New("EVM coin denom and extended denom must be the same for 18 decimals")
		}
	}

	evmCoinInfo = new(EvmCoinInfo)

	if err := setEVMCoinDenom(eci.Denom); err != nil {
		return err
	}
	if err := setEVMCoinExtendedDenom(eci.ExtendedDenom); err != nil {
		return err
	}
	return setEVMCoinDecimals(eci.Decimals)
}
