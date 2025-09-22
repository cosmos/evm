package types

import (
	"fmt"
	"slices"

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
type Decimals uint8

// Validate checks if the Decimals instance represent a supported decimals value
func (d Decimals) Validate() error {
	validDecimals := []Decimals{
		OneDecimals,
		TwoDecimals,
		ThreeDecimals,
		SixDecimals,
		NineDecimals,
		TwelveDecimals,
		FifteenDecimals,
		EighteenDecimals,
	}

	if slices.Contains(validDecimals, d) {
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

// GetSIPrefix returns the SI prefix for the decimals
func (d Decimals) GetSIPrefix() string {
	switch d {
	case OneDecimals:
		return "d"
	case TwoDecimals:
		return "c"
	case ThreeDecimals:
		return "m"
	case SixDecimals:
		return "u"
	case NineDecimals:
		return "n"
	case TwelveDecimals:
		return "p"
	case FifteenDecimals:
		return "f"
	case EighteenDecimals:
		return "a"
	default:
		// decimals must be one of 1, 2, 3, 6, 9, 12, 15, 18 to have a valid prefix
		return "invalid"
	}
}

func DecimalsFromSIPrefix(prefix string) (Decimals, error) {
	switch prefix {
	case "d":
		return OneDecimals, nil
	case "c":
		return TwoDecimals, nil
	case "m":
		return ThreeDecimals, nil
	case "u":
		return SixDecimals, nil
	case "n":
		return NineDecimals, nil
	case "p":
		return TwelveDecimals, nil
	case "f":
		return FifteenDecimals, nil
	case "a":
		return EighteenDecimals, nil
	default:
		return 0, fmt.Errorf("invalid SI prefix: %s", prefix)
	}
}
