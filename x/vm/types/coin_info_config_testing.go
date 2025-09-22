//go:build test
// +build test

package types

import (
	"errors"
	"fmt"
	"sync"
)

// testingEvmCoinInfo hold the information of the coin used in the EVM as gas token. It
// can only be set via `EvmConfig.Apply` before starting the app.
var (
	testingEvmCoinInfo     *EvmCoinInfo
	testingEvmCoinInfoOnce sync.Once
)

// GetEVMCoinDisplayDenom returns the display denom used for the EVM coin.
func GetEVMCoinDisplayDenom() string {
	return testingEvmCoinInfo.DisplayDenom
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

// SetTestingEVMCoinInfo allows to define denom and decimals of the coin used in the EVM.
func SetTestingEVMCoinInfo(eci EvmCoinInfo) error {
	if testingEvmCoinInfo != nil {
		return errors.New("testing EVM coin info already set. Make sure you run the configurator's ResetTestConfig before trying to set a new evm coin info")
	}

	if eci.Decimals == EighteenDecimals {
		if eci.Decimals != eci.ExtendedDecimals {
			return errors.New("EVM coin decimals and extended decimals must be the same for 18 decimals")
		}
	}

	if err := eci.Validate(); err != nil {
		return fmt.Errorf("validation failed for evm coin info: %w", err)
	}

	testingEvmCoinInfoOnce.Do(func() {
		testingEvmCoinInfo = new(EvmCoinInfo)
		testingEvmCoinInfo.DisplayDenom = eci.DisplayDenom
		testingEvmCoinInfo.Decimals = eci.Decimals
		testingEvmCoinInfo.ExtendedDecimals = eci.ExtendedDecimals
	})

	return nil
}

// resetEVMCoinInfo resets to nil the testingEVMCoinInfo
func resetEVMCoinInfo() {
	testingEvmCoinInfo = nil
	testingEvmCoinInfoOnce = sync.Once{}
}
