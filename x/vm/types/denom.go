//
// The config package provides a convenient way to modify x/evm params and values.
// Its primary purpose is to be used during application initialization.

package types

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
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

var ConversionFactor = map[Decimals]sdkmath.Int{
	OneDecimals:       sdkmath.NewInt(1e17),
	TwoDecimals:       sdkmath.NewInt(1e16),
	ThreeDecimals:     sdkmath.NewInt(1e15),
	FourDecimals:      sdkmath.NewInt(1e14),
	FiveDecimals:      sdkmath.NewInt(1e13),
	SixDecimals:       sdkmath.NewInt(1e12),
	SevenDecimals:     sdkmath.NewInt(1e11),
	EightDecimals:     sdkmath.NewInt(1e10),
	NineDecimals:      sdkmath.NewInt(1e9),
	TenDecimals:       sdkmath.NewInt(1e8),
	ElevenDecimals:    sdkmath.NewInt(1e7),
	TwelveDecimals:    sdkmath.NewInt(1e6),
	ThirteenDecimals:  sdkmath.NewInt(1e5),
	FourteenDecimals:  sdkmath.NewInt(1e4),
	FifteenDecimals:   sdkmath.NewInt(1e3),
	SixteenDecimals:   sdkmath.NewInt(1e2),
	SeventeenDecimals: sdkmath.NewInt(1e1),
	EighteenDecimals:  sdkmath.NewInt(1e0),
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
func (d Decimals) ConversionFactor() sdkmath.Int {
	return ConversionFactor[d]
}

// EvmCoinInfo struct holds the name and decimals of the EVM denom. The EVM denom
// is the token used to pay fees in the EVM.
//
// TODO: move to own file? at least rename file because it's unclear to use "denom"
type EvmCoinInfo struct {
	Denom        string
	DisplayDenom string
	Decimals     Decimals
}
