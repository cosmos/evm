//
// The config package provides a convenient way to modify x/evm params and values.
// Its primary purpose is to be used during application initialization.

package types

import (
	"errors"
	"fmt"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NOTE: Remember to add the ConversionFactor associated with constants.
const (
	OneDecimals       Decimals = 1
	TwoDecimals       Decimals = 2
	ThreeDecimals     Decimals = 3
	FourDecimals      Decimals = 4
	FiveDecimals      Decimals = 5
	SixDecimals       Decimals = 6 // SixDecimals is the Decimals used for Cosmos coin with 6 decimals.
	SevenDecimals     Decimals = 7
	EightDecimals     Decimals = 8
	NineDecimals      Decimals = 9
	TenDecimals       Decimals = 10
	ElevenDecimals    Decimals = 11
	TwelveDecimals    Decimals = 12
	ThirteenDecimals  Decimals = 13
	FourteenDecimals  Decimals = 14
	FifteenDecimals   Decimals = 15
	SixteenDecimals   Decimals = 16
	SeventeenDecimals Decimals = 17
	EighteenDecimals  Decimals = 18 // EighteenDecimals is the Decimals used for Cosmos coin with 18 decimals.
)

var ConversionFactor = map[Decimals]math.Int{
	OneDecimals:       math.NewInt(1e17),
	TwoDecimals:       math.NewInt(1e16),
	ThreeDecimals:     math.NewInt(1e15),
	FourDecimals:      math.NewInt(1e14),
	FiveDecimals:      math.NewInt(1e13),
	SixDecimals:       math.NewInt(1e12),
	SevenDecimals:     math.NewInt(1e11),
	EightDecimals:     math.NewInt(1e10),
	NineDecimals:      math.NewInt(1e9),
	TenDecimals:       math.NewInt(1e8),
	ElevenDecimals:    math.NewInt(1e7),
	TwelveDecimals:    math.NewInt(1e6),
	ThirteenDecimals:  math.NewInt(1e5),
	FourteenDecimals:  math.NewInt(1e4),
	FifteenDecimals:   math.NewInt(1e3),
	SixteenDecimals:   math.NewInt(1e2),
	SeventeenDecimals: math.NewInt(1e1),
	EighteenDecimals:  math.NewInt(1e0),
}

// Decimals represents the decimal representation of a Cosmos coin.
type Decimals uint8

// Validate checks if the Decimals instance represent a supported decimals value
// or not.
func (d Decimals) Validate() error {
	if 0 < d && d <= EighteenDecimals {
		return nil
	}

	return fmt.Errorf("received unsupported decimals: %d", d)
}

// ConversionFactor returns the conversion factor between the Decimals value and
// the 18 decimals representation, i.e. `EighteenDecimals`.
//
// NOTE: This function does not check if the Decimal instance is valid or
// not and by default returns the conversion factor of 1, i.e. from 18 decimals
// to 18 decimals. We cannot have a non supported Decimal since it is checked
// and validated.
func (d Decimals) ConversionFactor() math.Int {
	return ConversionFactor[d]
}

// EvmCoinInfo struct holds the name and decimals of the EVM denom. The EVM denom
// is the token used to pay fees in the EVM.
//
// TODO: move to own file? at least rename file because it's unclear to use "denom"
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
