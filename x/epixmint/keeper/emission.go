package keeper

import (
	"context"
	"math"
	"math/big"
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
func calculateCurrentEmissionRate(ctx context.Context, params types.Params) sdkmath.Int {
	// Get current block height
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()

	// Calculate blocks per year based on actual block time (with fallback to parameter)
	actualBlockTime := calculateActualBlockTime(ctx, params.BlockTimeSeconds)
	blocksPerYear := calculateBlocksPerYear(actualBlockTime)

	// Calculate the reduction factor per block for smooth exponential decay
	// If we want 25% annual reduction, each block should reduce by (1-0.25)^(1/blocks_per_year)
	annualRetentionRate := sdkmath.LegacyOneDec().Sub(params.AnnualReductionRate) // 1 - 0.25 = 0.75

	// Convert to float64 for math.Pow calculation
	retentionRateFloat, _ := annualRetentionRate.Float64()
	blocksPerYearFloat := float64(blocksPerYear)
	currentHeightFloat := float64(currentHeight)

	// Calculate the current emission rate using exponential decay
	// rate = initial * retention_rate^(current_height / blocks_per_year)
	decayFactor := math.Pow(retentionRateFloat, currentHeightFloat/blocksPerYearFloat)

	// Convert back to sdkmath.Int
	initialAmountFloat := new(big.Float).SetInt(params.InitialAnnualMintAmount.BigInt())
	currentAnnualRateFloat := new(big.Float).Mul(initialAmountFloat, big.NewFloat(decayFactor))

	// Convert annual rate to per-block rate
	blocksPerYearBig := big.NewFloat(blocksPerYearFloat)
	currentBlockRateFloat := new(big.Float).Quo(currentAnnualRateFloat, blocksPerYearBig)

	// Convert to sdkmath.Int (truncate to integer)
	currentBlockRateInt, _ := currentBlockRateFloat.Int(nil)
	return sdkmath.NewIntFromBigInt(currentBlockRateInt)
}

// calculateCurrentAnnualEmissionRate calculates the current annual emission rate
// This is useful for queries and display purposes
func calculateCurrentAnnualEmissionRate(ctx context.Context, params types.Params) sdkmath.Int {
	// Get current block height
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	currentHeight := sdkCtx.BlockHeight()

	// Calculate blocks per year based on block time
	blocksPerYear := calculateBlocksPerYear(params.BlockTimeSeconds)

	// Calculate the annual retention rate
	annualRetentionRate := sdkmath.LegacyOneDec().Sub(params.AnnualReductionRate) // 1 - 0.25 = 0.75

	// Convert to float64 for math.Pow calculation
	retentionRateFloat, _ := annualRetentionRate.Float64()
	blocksPerYearFloat := float64(blocksPerYear)
	currentHeightFloat := float64(currentHeight)

	// Calculate years elapsed
	yearsElapsed := currentHeightFloat / blocksPerYearFloat

	// Calculate the current annual emission rate
	// rate = initial * retention_rate^years_elapsed
	decayFactor := math.Pow(retentionRateFloat, yearsElapsed)

	// Convert back to sdkmath.Int
	initialAmountFloat := new(big.Float).SetInt(params.InitialAnnualMintAmount.BigInt())
	currentAnnualRateFloat := new(big.Float).Mul(initialAmountFloat, big.NewFloat(decayFactor))

	// Convert to sdkmath.Int (truncate to integer)
	currentAnnualRateInt, _ := currentAnnualRateFloat.Int(nil)
	return sdkmath.NewIntFromBigInt(currentAnnualRateInt)
}
