package utils

import (
	"math/big"

	"cosmossdk.io/math"
)

var (
	// AttoPowerReduction defines the power reduction for att units (1e18)
	AttoPowerReduction = math.NewIntFromBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))

	// MicroPowerReduction defines the power reduction for micro units (1e6)
	MicroPowerReduction = math.NewIntFromBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(6), nil))
)
