//
// The config package provides a convenient way to modify x/evm params and values.
// Its primary purpose is to be used during application initialization.

package types

import (
	"errors"
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EvmCoinInfo struct holds the display name and decimal precisions of the EVM denom
type EvmCoinInfo struct {
	// DisplayDenom defines the display denomination shown to users
	DisplayDenom string `mapstructure:"display-denom"`
	// Decimals defines the precision/decimals for the base denomination (1-18)
	Decimals Decimals `mapstructure:"decimals"`
	// ExtendedDecimals defines the precision/decimals for the extended denomination (typically 18 decimals for atto-denom)
	ExtendedDecimals Decimals `mapstructure:"extended-decimals"`
}

// Validate returns an error if the coin configuration fields are invalid.
func (c EvmCoinInfo) Validate() error {
	if err := c.Decimals.Validate(); err != nil {
		return fmt.Errorf("decimals validation failed: %w", err)
	}
	if err := c.ExtendedDecimals.Validate(); err != nil {
		return fmt.Errorf("extended decimals validation failed: %w", err)
	}

	denom := c.GetDenom()
	if strings.HasPrefix(denom, "invalid") {
		return errors.New("invalid denom, not a valid SI decimal, so denom cannot be derived")
	}
	if err := sdk.ValidateDenom(denom); err != nil {
		return fmt.Errorf("invalid denom: %w", err)
	}

	extendedDenom := c.GetExtendedDenom()
	if strings.HasPrefix(extendedDenom, "invalid") {
		return errors.New("invalid extended denom, not a valid SI decimal, so extended denom cannot be derived")
	}
	if err := sdk.ValidateDenom(extendedDenom); err != nil {
		return fmt.Errorf("invalid extended denom: %w", err)
	}

	if c.DisplayDenom == "" {
		return errors.New("display-denom cannot be empty")
	}
	if err := sdk.ValidateDenom(c.DisplayDenom); err != nil {
		return fmt.Errorf("invalid display denom: %w", err)
	}

	// For 18 decimals, denom and extended denom should be the same, as higher decimals are not supported
	if c.Decimals == EighteenDecimals {
		if c.Decimals != c.ExtendedDecimals {
			return errors.New("decimals and extended decimals must be the same for 18 decimals")
		}
		if c.GetDenom() != c.GetExtendedDenom() {
			return errors.New("denom and extended denom must be the same for 18 decimals")
		}
	}

	return nil
}

// GetDenom returns the base denomination used in the chain, derived by SI prefix
func (c EvmCoinInfo) GetDenom() string {
	return CreateDenomStr(c.Decimals, c.DisplayDenom)
}

// GetExtendedDenom returns the extended denomination used in the chain, derived by SI prefix
func (c EvmCoinInfo) GetExtendedDenom() string {
	return CreateDenomStr(c.ExtendedDecimals, c.DisplayDenom)
}

func CreateDenomStr(decimals Decimals, displayDenom string) string {
	return decimals.GetSIPrefix() + displayDenom
}
