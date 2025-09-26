//
// The config package provides a convenient way to modify x/evm params and values.
// Its primary purpose is to be used during application initialization.

package types

import (
	"fmt"

	"cosmossdk.io/math"
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
type Decimals uint32

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

func (d Decimals) Uint32() uint32 { return uint32(d) }
