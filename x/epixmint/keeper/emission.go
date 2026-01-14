package keeper

import (
	"context"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/evm/x/epixmint/types"
)

// MaxDecayYears is the maximum number of years to calculate decay for.
// After this point, emission continues at a constant "tail emission" rate
// (the rate at MaxDecayYears) until max supply is reached.
// This ensures the chain eventually reaches the 42B max supply cap.
const MaxDecayYears = 20

// calculateBlocksPerYear calculates the number of blocks per year based on block time
func calculateBlocksPerYear(blockTimeSeconds uint64) uint64 {
	secondsPerYear := uint64(365 * 24 * 60 * 60) // 31,536,000 seconds
	return secondsPerYear / blockTimeSeconds
}

// calculateDecayFactorAndBlocksPerYear is a helper that computes the decay factor
// and blocks per year from the current context and params.
// This centralizes the common logic used by both emission rate functions.
func calculateDecayFactorAndBlocksPerYear(ctx context.Context, params types.Params) (decayFactor sdkmath.LegacyDec, blocksPerYearDec sdkmath.LegacyDec) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()

	// Calculate blocks per year based on configured block time
	blocksPerYear := calculateBlocksPerYear(params.BlockTimeSeconds)
	blocksPerYearDec = sdkmath.LegacyNewDec(int64(blocksPerYear))
	currentHeightDec := sdkmath.LegacyNewDec(currentHeight)

	// Calculate years elapsed (can be fractional)
	yearsElapsed := currentHeightDec.Quo(blocksPerYearDec)

	// Calculate the annual retention rate: 1 - reduction_rate (e.g., 1 - 0.25 = 0.75)
	annualRetentionRate := sdkmath.LegacyOneDec().Sub(params.AnnualReductionRate)

	// Calculate decay factor using deterministic power approximation
	decayFactor = ApproximateDecayWithDec(annualRetentionRate, yearsElapsed)

	return decayFactor, blocksPerYearDec
}

// calculateCurrentEmissionRate calculates the current emission rate per block
// using smooth exponential decay: rate = initial * (1 - reduction_rate)^(blocks_elapsed / blocks_per_year)
// Uses deterministic sdkmath.LegacyDec arithmetic to ensure consensus across all architectures.
func calculateCurrentEmissionRate(ctx context.Context, params types.Params) sdkmath.Int {
	decayFactor, blocksPerYearDec := calculateDecayFactorAndBlocksPerYear(ctx, params)

	// Calculate current annual rate: initial * decayFactor
	initialAmount := sdkmath.LegacyNewDecFromInt(params.InitialAnnualMintAmount)
	currentAnnualRate := initialAmount.Mul(decayFactor)

	// Convert annual rate to per-block rate
	currentBlockRate := currentAnnualRate.Quo(blocksPerYearDec)

	return currentBlockRate.TruncateInt()
}

// calculateCurrentAnnualEmissionRate calculates the current annual emission rate.
// This is useful for queries and display purposes.
// Uses deterministic sdkmath.LegacyDec arithmetic to ensure consensus across all architectures.
func calculateCurrentAnnualEmissionRate(ctx context.Context, params types.Params) sdkmath.Int {
	decayFactor, _ := calculateDecayFactorAndBlocksPerYear(ctx, params)

	// Calculate current annual rate: initial * decayFactor
	initialAmount := sdkmath.LegacyNewDecFromInt(params.InitialAnnualMintAmount)
	currentAnnualRate := initialAmount.Mul(decayFactor)

	return currentAnnualRate.TruncateInt()
}

// ApproximateDecayWithDec calculates base^exp using deterministic integer arithmetic.
// This function handles both the integer part of the exponent (using iterative multiplication)
// and the fractional part (using linear interpolation for smoothness).
// This ensures consensus across all CPU architectures (x86, ARM, etc.)
//
// Parameters:
//   - base: Must be between 0 and 1 (exclusive of 0, inclusive of 1) for decay behavior.
//     If base > 1, the result will grow instead of decay.
//   - exp: The exponent (years elapsed). Capped at MaxDecayYears for performance.
//
// The function is exported to allow testing and verification of deterministic behavior.
func ApproximateDecayWithDec(base sdkmath.LegacyDec, exp sdkmath.LegacyDec) sdkmath.LegacyDec {
	// Handle edge cases
	if exp.IsZero() {
		return sdkmath.LegacyOneDec()
	}
	if exp.IsNegative() {
		// For negative exponents, invert the result
		return sdkmath.LegacyOneDec().Quo(ApproximateDecayWithDec(base, exp.Neg()))
	}

	// Validate base is in valid range for decay (0 < base <= 1)
	// If base <= 0, return zero to prevent invalid calculations
	if base.IsNegative() || base.IsZero() {
		return sdkmath.LegacyZeroDec()
	}
	// If base > 1, this is growth not decay - still valid mathematically but log a note
	// We allow it to proceed as it may be intentional

	// Split exponent into integer and fractional parts
	wholeYears := exp.TruncateInt().Int64()
	fractionalPart := exp.Sub(sdkmath.LegacyNewDec(wholeYears))

	// Cap whole years to prevent slow loops
	if wholeYears > MaxDecayYears {
		wholeYears = MaxDecayYears
		fractionalPart = sdkmath.LegacyZeroDec() // Ignore fractional part when capped
	}

	// Calculate base^wholeYears using iterative multiplication (deterministic)
	result := sdkmath.LegacyOneDec()
	for i := int64(0); i < wholeYears; i++ {
		result = result.Mul(base)
	}

	// For the fractional part, use linear interpolation between year boundaries
	// This provides a smooth decay curve while remaining deterministic
	// Linear interpolation: result * (1 - fractionalPart + fractionalPart * base)
	// This is equivalent to: result * ((1 - fractionalPart) + fractionalPart * base)
	if !fractionalPart.IsZero() {
		// Calculate: (1 - frac) + frac * base = 1 - frac + frac * base = 1 + frac * (base - 1)
		interpolation := sdkmath.LegacyOneDec().Add(fractionalPart.Mul(base.Sub(sdkmath.LegacyOneDec())))
		result = result.Mul(interpolation)
	}

	return result
}
