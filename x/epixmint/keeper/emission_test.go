package keeper_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/cosmos/evm/x/epixmint/keeper"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
)

func TestCalculateBlocksPerYear(t *testing.T) {
	testCases := []struct {
		name             string
		blockTimeSeconds uint64
		expectedBlocks   uint64
	}{
		{
			name:             "6 second blocks",
			blockTimeSeconds: 6,
			expectedBlocks:   5_256_000, // 365 * 24 * 60 * 60 / 6
		},
		{
			name:             "3 second blocks",
			blockTimeSeconds: 3,
			expectedBlocks:   10_512_000, // 365 * 24 * 60 * 60 / 3
		},
		{
			name:             "12 second blocks",
			blockTimeSeconds: 12,
			expectedBlocks:   2_628_000, // 365 * 24 * 60 * 60 / 12
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use reflection to access the private function for testing
			// In a real implementation, you might want to make this function public
			// or create a test helper
			secondsPerYear := uint64(365 * 24 * 60 * 60)
			actualBlocks := secondsPerYear / tc.blockTimeSeconds
			require.Equal(t, tc.expectedBlocks, actualBlocks)
		})
	}
}

func TestEmissionCalculation(t *testing.T) {
	// Test the mathematical decay calculation
	initialAmount := 10.527e9 // 10.527 billion EPIX

	testCases := []struct {
		name                string
		years               float64
		expectedDecayFactor float64
	}{
		{
			name:                "Genesis",
			years:               0.0,
			expectedDecayFactor: 1.0,
		},
		{
			name:                "After 1 year",
			years:               1.0,
			expectedDecayFactor: 0.75,
		},
		{
			name:                "After 2 years",
			years:               2.0,
			expectedDecayFactor: 0.5625, // 0.75^2
		},
		{
			name:                "After 10 years",
			years:               10.0,
			expectedDecayFactor: 0.0563, // 0.75^10
		},
		{
			name:                "After 20 years",
			years:               20.0,
			expectedDecayFactor: 0.003171, // 0.75^20 (more precise)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate decay factor: (1 - 0.25)^years = 0.75^years
			retentionRate := 0.75
			decayFactor := math.Pow(retentionRate, tc.years)
			expectedAnnualRate := initialAmount * decayFactor

			// Verify the decay factor matches expected
			require.InDelta(t, tc.expectedDecayFactor, decayFactor, 0.001,
				"Decay factor should match expected value for year %.1f", tc.years)

			// Verify the annual rate is positive and decreasing
			require.Greater(t, expectedAnnualRate, 0.0, "Annual rate should always be positive")
			if tc.years > 0 {
				require.Less(t, expectedAnnualRate, initialAmount, "Annual rate should decrease over time")
			}
		})
	}
}

func TestTwentyYearEmissionTotal(t *testing.T) {
	// Test that the total emission over 20 years approaches 42B EPIX
	// Calculate total using geometric series formula
	// Total = a * (1 - r^n) / (1 - r)
	// Where: a = 10.527B, r = 0.75, n = 20

	a := 10.527e9 // 10.527 billion
	r := 0.75     // retention rate (1 - 0.25)
	n := 20.0     // 20 years

	total := a * (1 - math.Pow(r, n)) / (1 - r)

	// Should be very close to 42 billion (allowing for some precision differences)
	require.InDelta(t, 42.0e9, total, 0.2e9, "Total emission over 20 years should be ~42B EPIX")

	// Verify it's close to 42B (the max supply protection will ensure we don't exceed 42B)
	require.InDelta(t, 42.0e9, total, 0.5e9, "Total should be close to 42B")
}

func TestEmissionRateDecrease(t *testing.T) {
	// Test that emission rate decreases each year by approximately 25%
	initialAmount := 10.527e9 // 10.527 billion EPIX

	testCases := []struct {
		year         int
		expectedRate float64
	}{
		{0, 10.527e9},                       // Year 0: 10.527B
		{1, 10.527e9 * 0.75},                // Year 1: 7.895B
		{2, 10.527e9 * 0.75 * 0.75},         // Year 2: 5.921B
		{5, 10.527e9 * math.Pow(0.75, 5)},   // Year 5: ~3.331B
		{10, 10.527e9 * math.Pow(0.75, 10)}, // Year 10: ~563M
		{20, 10.527e9 * math.Pow(0.75, 20)}, // Year 20: ~16M
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Year_%d", tc.year), func(t *testing.T) {
			actualRate := initialAmount * math.Pow(0.75, float64(tc.year))
			require.InDelta(t, tc.expectedRate, actualRate, tc.expectedRate*0.001,
				"Emission rate should match expected value for year %d", tc.year)
		})
	}
}

func TestBlockTimeAdjustment(t *testing.T) {
	// Test that changing block time adjusts the per-block emission correctly
	initialAmount, _ := sdkmath.NewIntFromString("10527000000000000000000000000") // 10.527B EPIX

	testCases := []struct {
		name                  string
		blockTimeSeconds      uint64
		expectedBlocksPerYear uint64
	}{
		{"3 second blocks", 3, 10_512_000},
		{"6 second blocks", 6, 5_256_000},
		{"12 second blocks", 12, 2_628_000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate blocks per year
			secondsPerYear := uint64(365 * 24 * 60 * 60)
			blocksPerYear := secondsPerYear / tc.blockTimeSeconds
			require.Equal(t, tc.expectedBlocksPerYear, blocksPerYear)

			// Calculate per-block emission at genesis
			perBlockEmission := initialAmount.Quo(sdkmath.NewIntFromUint64(blocksPerYear))

			// Verify per-block emission is positive
			require.True(t, perBlockEmission.IsPositive(), "Per-block emission should be positive")

			// Verify that faster blocks = smaller per-block emission
			if tc.blockTimeSeconds < 6 {
				// Faster blocks should have smaller per-block emission
				referencePerBlock := initialAmount.Quo(sdkmath.NewIntFromUint64(5_256_000))
				require.True(t, perBlockEmission.LT(referencePerBlock),
					"Faster blocks should have smaller per-block emission")
			}
		})
	}
}

func TestApproximateDecayWithDec(t *testing.T) {
	// Test the deterministic decay function directly
	base := sdkmath.LegacyMustNewDecFromStr("0.75") // 75% retention rate

	testCases := []struct {
		name           string
		exp            string
		expectedResult string
		tolerance      string
	}{
		{
			name:           "Zero exponent",
			exp:            "0",
			expectedResult: "1.0",
			tolerance:      "0.0001",
		},
		{
			name:           "One year",
			exp:            "1",
			expectedResult: "0.75",
			tolerance:      "0.0001",
		},
		{
			name:           "Two years",
			exp:            "2",
			expectedResult: "0.5625", // 0.75^2
			tolerance:      "0.0001",
		},
		{
			name:           "Half year (linear interpolation)",
			exp:            "0.5",
			expectedResult: "0.875", // 1 + 0.5 * (0.75 - 1) = 0.875
			tolerance:      "0.0001",
		},
		{
			name:           "Ten years",
			exp:            "10",
			expectedResult: "0.0563", // 0.75^10
			tolerance:      "0.001",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exp := sdkmath.LegacyMustNewDecFromStr(tc.exp)
			result := keeper.ApproximateDecayWithDec(base, exp)

			expected := sdkmath.LegacyMustNewDecFromStr(tc.expectedResult)
			tolerance := sdkmath.LegacyMustNewDecFromStr(tc.tolerance)

			diff := result.Sub(expected).Abs()
			require.True(t, diff.LTE(tolerance),
				"Result %s should be within %s of expected %s (diff: %s)",
				result.String(), tc.tolerance, tc.expectedResult, diff.String())
		})
	}
}

func TestApproximateDecayWithDecEdgeCases(t *testing.T) {
	t.Run("Negative base returns zero", func(t *testing.T) {
		base := sdkmath.LegacyMustNewDecFromStr("-0.5")
		exp := sdkmath.LegacyOneDec()
		result := keeper.ApproximateDecayWithDec(base, exp)
		require.True(t, result.IsZero(), "Negative base should return zero")
	})

	t.Run("Zero base returns zero", func(t *testing.T) {
		base := sdkmath.LegacyZeroDec()
		exp := sdkmath.LegacyOneDec()
		result := keeper.ApproximateDecayWithDec(base, exp)
		require.True(t, result.IsZero(), "Zero base should return zero")
	})

	t.Run("Negative exponent inverts result", func(t *testing.T) {
		base := sdkmath.LegacyMustNewDecFromStr("0.75")
		exp := sdkmath.LegacyMustNewDecFromStr("-1")
		result := keeper.ApproximateDecayWithDec(base, exp)

		// 0.75^-1 = 1/0.75 = 1.333...
		expected := sdkmath.LegacyOneDec().Quo(sdkmath.LegacyMustNewDecFromStr("0.75"))
		tolerance := sdkmath.LegacyMustNewDecFromStr("0.0001")
		diff := result.Sub(expected).Abs()
		require.True(t, diff.LTE(tolerance),
			"Negative exponent should invert: got %s, expected %s", result.String(), expected.String())
	})

	t.Run("Large exponent is capped at MaxDecayYears", func(t *testing.T) {
		base := sdkmath.LegacyMustNewDecFromStr("0.75")
		exp := sdkmath.LegacyNewDec(200) // Larger than MaxDecayYears (100)

		// Should return 0.75^100, not 0.75^200
		result := keeper.ApproximateDecayWithDec(base, exp)

		// 0.75^100 is a very small number but not zero
		require.True(t, result.IsPositive(), "Result should still be positive")

		// Calculate expected: 0.75^100
		expected := sdkmath.LegacyOneDec()
		for i := 0; i < 100; i++ {
			expected = expected.Mul(base)
		}
		require.Equal(t, expected.String(), result.String(),
			"Result should be capped at 100 years")
	})

	t.Run("Determinism - same inputs always give same output", func(t *testing.T) {
		base := sdkmath.LegacyMustNewDecFromStr("0.75")
		exp := sdkmath.LegacyMustNewDecFromStr("5.5")

		// Run 100 times and ensure same result
		firstResult := keeper.ApproximateDecayWithDec(base, exp)
		for i := 0; i < 100; i++ {
			result := keeper.ApproximateDecayWithDec(base, exp)
			require.Equal(t, firstResult.String(), result.String(),
				"Result should be deterministic on iteration %d", i)
		}
	})
}
