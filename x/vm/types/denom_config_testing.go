//go:build test
// +build test

package types

import (
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// testingEvmCoinInfo hold the information of the coin used in the EVM as gas token. It
// can only be set via `EvmConfig.Apply` before starting the app.
var testingEvmCoinInfo *EvmCoinInfo

// setEVMCoinDecimals allows to define the decimals used in the representation
// of the EVM coin.
func setEVMCoinDecimals(d Decimals) error {
	if err := d.Validate(); err != nil {
		return fmt.Errorf("setting EVM coin decimals: %w", err)
	}

	testingEvmCoinInfo.Decimals = d
	return nil
}

// setEVMCoinExtendedDecimals allows to define the extended denom of the coin used in the EVM.
func setEVMCoinExtendedDecimals(d Decimals) error {
	if err := d.Validate(); err != nil {
		return err
	}
	testingEvmCoinInfo.ExtendedDecimals = d
	return nil
}

func setDisplayDenom(displayDenom string) error {
	if err := sdk.ValidateDenom(displayDenom); err != nil {
		return fmt.Errorf("setting EVM coin display denom: %w", err)
	}
	testingEvmCoinInfo.DisplayDenom = displayDenom
	return nil
}

// GetEVMCoinDecimals returns the decimals used in the representation of the EVM
// coin.
func GetEVMCoinDecimals() Decimals {
	return testingEvmCoinInfo.Decimals
}

// GetEVMCoinExtendedDecimals returns the extended decimals used in the
// representation of the EVM coin.
func GetEVMCoinExtendedDecimals() Decimals {
	return testingEvmCoinInfo.ExtendedDecimals
}

// GetEVMCoinDenom returns the denom used for the EVM coin.
func GetEVMCoinDenom() string {
	return testingEvmCoinInfo.GetDenom()
}

// GetEVMCoinExtendedDenom returns the extended denom used for the EVM coin.
func GetEVMCoinExtendedDenom() string {
	return testingEvmCoinInfo.GetExtendedDenom()
}

// GetEVMCoinDisplayDenom returns the display denom used for the EVM coin.
func GetEVMCoinDisplayDenom() string {
	return testingEvmCoinInfo.DisplayDenom
}

// setTestingEVMCoinInfo allows to define denom and decimals of the coin used in the EVM.
func setTestingEVMCoinInfo(eci EvmCoinInfo) error {
	if testingEvmCoinInfo != nil {
		return errors.New("testing EVM coin info already set. Make sure you run the configurator's ResetTestConfig before trying to set a new evm coin info")
	}

	if eci.Decimals == EighteenDecimals {
		if eci.Decimals != eci.ExtendedDecimals {
			return errors.New("EVM coin decimals and extended decimals must be the same for 18 decimals")
		}
	}

	testingEvmCoinInfo = new(EvmCoinInfo)

	if err := setEVMCoinDecimals(eci.Decimals); err != nil {
		return err
	}
	if err := setEVMCoinExtendedDecimals(eci.ExtendedDecimals); err != nil {
		return err
	}
	return setDisplayDenom(eci.DisplayDenom)
}

// resetEVMCoinInfo resets to nil the testingEVMCoinInfo
func resetEVMCoinInfo() {
	testingEvmCoinInfo = nil
}
