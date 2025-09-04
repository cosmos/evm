//
// The config package provides a convenient way to modify x/evm params and values.
// Its primary purpose is to be used during application initialization.

package types

import (
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EvmCoinInfo struct holds the name and decimals of the EVM denom. The EVM denom
// is the token used to pay fees in the EVM.
type EvmCoinInfo struct {
	// Denom defines the base denomination used in the chain
	Denom string `mapstructure:"denom"`
	// ExtendedDenom defines the extended denomination (typically atto-denom for 18 decimals)
	ExtendedDenom string `mapstructure:"extended-denom"`
	// DisplayDenom defines the display denomination shown to users
	DisplayDenom string `mapstructure:"display-denom"`
	// Decimals defines the precision/decimals for the base denomination (1-18)
	Decimals Decimals `mapstructure:"decimals"`
}

// Validate returns an error if the coin configuration fields are invalid.
func (c EvmCoinInfo) Validate() error {
	if c.Denom == "" {
		return errors.New("denom cannot be empty")
	}
	if err := sdk.ValidateDenom(c.Denom); err != nil {
		return fmt.Errorf("invalid denom: %w", err)
	}

	if c.ExtendedDenom == "" {
		return errors.New("extended-denom cannot be empty")
	}
	if err := sdk.ValidateDenom(c.ExtendedDenom); err != nil {
		return fmt.Errorf("invalid extended-denom: %w", err)
	}

	if c.DisplayDenom == "" {
		return errors.New("display-denom cannot be empty")
	}
	if err := sdk.ValidateDenom(c.DisplayDenom); err != nil {
		return fmt.Errorf("invalid display-denom: %w", err)
	}

	if err := c.Decimals.Validate(); err != nil {
		return fmt.Errorf("decimals validation failed: %w", err)
	}

	// For 18 decimals, denom and extended denom should be the same
	if c.Decimals == EighteenDecimals && c.Denom != c.ExtendedDenom {
		return errors.New("denom and extended-denom must be the same for 18 decimals")
	}

	return nil
}
