//go:build !test
// +build !test

package types

import (
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// evmCoinInfo hold the information of the coin used in the EVM as gas token. It
// can only be set via `EvmConfig.Apply` before starting the app.
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

// setEVMCoinExtendedDecimals allows to define the extended denom of the coin used in the EVM.
func setEVMCoinExtendedDecimals(d Decimals) error {
	if err := d.Validate(); err != nil {
		return err
	}
	evmCoinInfo.ExtendedDecimals = d
	return nil
}

func setDisplayDenom(displayDenom string) error {
	if err := sdk.ValidateDenom(displayDenom); err != nil {
		return fmt.Errorf("setting EVM coin display denom: %w", err)
	}
	evmCoinInfo.DisplayDenom = displayDenom
	return nil
}

// GetEVMCoinDecimals returns the decimals used in the representation of the EVM
// coin.
func GetEVMCoinDecimals() Decimals {
	return evmCoinInfo.Decimals
}

// GetEVMCoinExtendedDecimals returns the extended decimals used in the
// representation of the EVM coin.
func GetEVMCoinExtendedDecimals() Decimals {
	return evmCoinInfo.ExtendedDecimals
}

// GetEVMCoinDenom returns the denom used for the EVM coin.
func GetEVMCoinDenom() string {
	return evmCoinInfo.GetDenom()
}

// GetEVMCoinExtendedDenom returns the extended denom used for the EVM coin.
func GetEVMCoinExtendedDenom() string {
	return evmCoinInfo.GetExtendedDenom()
}

// GetEVMCoinDisplayDenom returns the display denom used for the EVM coin.
func GetEVMCoinDisplayDenom() string {
	return evmCoinInfo.DisplayDenom
}

// setEVMCoinInfo allows to define denom and decimals of the coin used in the EVM.
func setEVMCoinInfo(eci EvmCoinInfo) error {
	if evmCoinInfo != nil {
		return errors.New("EVM coin info already set")
	}

	if eci.Decimals == EighteenDecimals {
		if eci.Decimals != eci.ExtendedDecimals {
			return errors.New("EVM coin decimals and extended decimals must be the same for 18 decimals")
		}
	}

	evmCoinInfo = new(EvmCoinInfo)

	if err := setEVMCoinDecimals(eci.Decimals); err != nil {
		return err
	}
	if err := setEVMCoinExtendedDecimals(eci.ExtendedDecimals); err != nil {
		return err
	}
	return setDisplayDenom(eci.DisplayDenom)
}
