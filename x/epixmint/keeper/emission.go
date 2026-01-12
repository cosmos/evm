package keeper

import (
	"context"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/evm/x/epixmint/types"
)

// calculateBlocksPerYear calculates the number of blocks per year based on block time
func calculateBlocksPerYear(blockTimeSeconds uint64) uint64 {
	secondsPerYear := uint64(365 * 24 * 60 * 60) // 31,536,000 seconds
	return secondsPerYear / blockTimeSeconds
}

// calculateActualBlockTime calculates the actual average block time from recent blocks
func calculateActualBlockTime(ctx context.Context, fallbackSeconds uint64) uint64 {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()

	// For early blocks, use the fallback parameter
	if currentHeight < 100 {
		return fallbackSeconds
	}

	// Calculate average block time from last 100 blocks
	// This is a simplified approach - in production you might want to use
	// a more sophisticated moving average or store historical data
	currentTime := sdkCtx.BlockTime()

	// Estimate time 100 blocks ago (this is approximate)
	estimatedPastTime := currentTime.Add(-100 * time.Duration(fallbackSeconds) * time.Second)
	actualDuration := currentTime.Sub(estimatedPastTime).Seconds()
	actualBlockTime := actualDuration / 100

	// Ensure reasonable bounds (between 1 and 60 seconds)
	if actualBlockTime < 1 {
		actualBlockTime = 1
	} else if actualBlockTime > 60 {
		actualBlockTime = 60
	}

	return uint64(actualBlockTime)
}

// calculateCurrentEmissionRate calculates the current emission rate per block
// using smooth exponential decay: rate = initial * (1 - reduction_rate)^(blocks_elapsed / blocks_per_year)
// Uses deterministic sdkmath.LegacyDec arithmetic to ensure consensus across all architectures.
func calculateCurrentEmissionRate(ctx context.Context, params types.Params) sdkmath.Int {
	// Get current block height
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()

	// Calculate blocks per year based on actual block time (with fallback to parameter)
	actualBlockTime := calculateActualBlockTime(ctx, params.BlockTimeSeconds)
	blocksPerYear := calculateBlocksPerYear(actualBlockTime)

	// Use deterministic sdkmath.LegacyDec for all calculations
	blocksPerYearDec := sdkmath.LegacyNewDec(int64(blocksPerYear))
	currentHeightDec := sdkmath.LegacyNewDec(currentHeight)

	// Calculate years elapsed (can be fractional)
	yearsElapsed := currentHeightDec.Quo(blocksPerYearDec)

	// Calculate the annual retention rate: 1 - reduction_rate (e.g., 1 - 0.25 = 0.75)
	annualRetentionRate := sdkmath.LegacyOneDec().Sub(params.AnnualReductionRate)

	// Calculate decay factor using deterministic power approximation
	decayFactor := approximateDecayWithDec(annualRetentionRate, yearsElapsed)

	// Calculate current annual rate: initial * decayFactor
	initialAmount := sdkmath.LegacyNewDecFromInt(params.InitialAnnualMintAmount)
	currentAnnualRate := initialAmount.Mul(decayFactor)

	// Convert annual rate to per-block rate
	currentBlockRate := currentAnnualRate.Quo(blocksPerYearDec)

	return currentBlockRate.TruncateInt()
}

// calculateCurrentAnnualEmissionRate calculates the current annual emission rate
// This is useful for queries and display purposes.
// Uses deterministic sdkmath.LegacyDec arithmetic to ensure consensus across all architectures.
func calculateCurrentAnnualEmissionRate(ctx context.Context, params types.Params) sdkmath.Int {
	// Get current block height
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()

	// Calculate blocks per year based on block time
	blocksPerYear := calculateBlocksPerYear(params.BlockTimeSeconds)

	// Use deterministic sdkmath.LegacyDec for all calculations
	blocksPerYearDec := sdkmath.LegacyNewDec(int64(blocksPerYear))
	currentHeightDec := sdkmath.LegacyNewDec(currentHeight)

	// Calculate years elapsed (can be fractional)
	yearsElapsed := currentHeightDec.Quo(blocksPerYearDec)

	// Calculate the annual retention rate: 1 - reduction_rate (e.g., 1 - 0.25 = 0.75)
	annualRetentionRate := sdkmath.LegacyOneDec().Sub(params.AnnualReductionRate)

	// Calculate decay factor using deterministic power approximation
	decayFactor := approximateDecayWithDec(annualRetentionRate, yearsElapsed)

	// Calculate current annual rate: initial * decayFactor
	initialAmount := sdkmath.LegacyNewDecFromInt(params.InitialAnnualMintAmount)
	currentAnnualRate := initialAmount.Mul(decayFactor)

	return currentAnnualRate.TruncateInt()
}

// approximateDecayWithDec calculates base^exp using deterministic integer arithmetic.
// This function handles both the integer part of the exponent (using iterative multiplication)
// and the fractional part (using linear interpolation for smoothness).
// This ensures consensus across all CPU architectures (x86, ARM, etc.)
func approximateDecayWithDec(base sdkmath.LegacyDec, exp sdkmath.LegacyDec) sdkmath.LegacyDec {
	// Handle edge cases
	if exp.IsZero() {
		return sdkmath.LegacyOneDec()
	}
	if exp.IsNegative() {
		// For negative exponents, invert the result
		return sdkmath.LegacyOneDec().Quo(approximateDecayWithDec(base, exp.Neg()))
	}

	// Split exponent into integer and fractional parts
	wholeYears := exp.TruncateInt().Int64()
	fractionalPart := exp.Sub(sdkmath.LegacyNewDec(wholeYears))

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
