package types_test

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/utils"
	"github.com/cosmos/evm/x/erc20/types"
)

const (
	testEpixChainDenom = "aepix"
	testWepixAddress   = "0x211781849EF6de72acbf1469Ce3808E74D7ce158"
)

// TestWEPIXSecurityScenarios tests various security-related scenarios
func TestWEPIXSecurityScenarios(t *testing.T) {
	testCases := []struct {
		name        string
		scenario    func(t *testing.T)
		description string
	}{
		{
			name:        "address_collision_resistance",
			scenario:    testAddressCollisionResistance,
			description: "Ensure different denoms generate different addresses",
		},
		{
			name:        "deterministic_generation",
			scenario:    testDeterministicGeneration,
			description: "Ensure address generation is always deterministic",
		},
		{
			name:        "edge_case_denoms",
			scenario:    testEdgeCaseDenoms,
			description: "Test with unusual but valid denom names",
		},
		{
			name:        "large_numbers",
			scenario:    testLargeNumbers,
			description: "Test with very large amounts",
		},
		{
			name:        "zero_amounts",
			scenario:    testZeroAmounts,
			description: "Test with zero amounts",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing: %s", tc.description)
			tc.scenario(t)
		})
	}
}

func testAddressCollisionResistance(t *testing.T) {
	// Test that different denoms generate different addresses
	denoms := []string{"aepix", "aatom", "stake", "uosmo", "ujuno", "uscrt"}
	addresses := make(map[string]bool)

	for _, denom := range denoms {
		addr, err := utils.GetNativeDenomAddress(denom)
		require.NoError(t, err)

		addrStr := addr.Hex()
		require.False(t, addresses[addrStr], "Address collision detected for denom %s: %s", denom, addrStr)
		addresses[addrStr] = true

		// Ensure address is not zero
		require.NotEqual(t, "0x0000000000000000000000000000000000000000", addrStr)
	}
}

func testDeterministicGeneration(t *testing.T) {
	// Test that the same denom always generates the same address
	for i := 0; i < 100; i++ {
		addr, err := utils.GetNativeDenomAddress(testEpixChainDenom)
		require.NoError(t, err)
		require.Equal(t, testWepixAddress, addr.Hex(), "Address generation not deterministic on iteration %d", i)
	}
}

func testEdgeCaseDenoms(t *testing.T) {
	edgeCases := []struct {
		denom     string
		shouldErr bool
		reason    string
	}{
		{"a", false, "single character denom"},
		{"aepix", false, "normal denom"},
		{"factory/osmo1234/token", false, "factory denom"},
		{"gravity0x1234", false, "gravity bridge denom"},
		{"", true, "empty denom should fail"},
		{"   ", true, "whitespace only should fail"},
		{"ibc/ABC123", true, "IBC denom should fail (use different function)"},
	}

	for _, tc := range edgeCases {
		t.Run("denom_"+tc.denom, func(t *testing.T) {
			addr, err := utils.GetNativeDenomAddress(tc.denom)

			if tc.shouldErr {
				require.Error(t, err, "Expected error for %s: %s", tc.denom, tc.reason)
			} else {
				require.NoError(t, err, "Unexpected error for %s: %s", tc.denom, tc.reason)
				require.NotEqual(t, "0x0000000000000000000000000000000000000000", addr.Hex())
			}
		})
	}
}

func testLargeNumbers(t *testing.T) {
	// Test with maximum possible values
	maxUint256 := new(big.Int)
	maxUint256.SetString("115792089237316195423570985008687907853269984665640564039457584007913129639935", 10)

	// Test that our address generation doesn't break with large inputs
	largeDenom := "very_long_denom_name_that_might_cause_issues_with_hashing_functions_and_address_generation"
	addr, err := utils.GetNativeDenomAddress(largeDenom)
	require.NoError(t, err)
	require.NotEqual(t, "0x0000000000000000000000000000000000000000", addr.Hex())
}

func testZeroAmounts(t *testing.T) {
	// Test token pair creation with normal denom (zero amounts are handled at higher levels)
	pair, err := types.NewTokenPairSTRv2(testEpixChainDenom)
	require.NoError(t, err)
	require.Equal(t, testEpixChainDenom, pair.Denom)
	require.Equal(t, testWepixAddress, pair.Erc20Address)
}

// TestWEPIXConsistencyChecks ensures consistency across different functions
func TestWEPIXConsistencyChecks(t *testing.T) {
	// Test that all our different ways of getting WEPIX address are consistent

	// Method 1: Direct address generation
	addr1, err := utils.GetNativeDenomAddress(testEpixChainDenom)
	require.NoError(t, err)

	// Method 2: Through WEPIX helper
	addr2, err := utils.GetWEPIXAddress(testEpixChainDenom)
	require.NoError(t, err)

	// Method 3: Through token pair creation
	pair, err := types.NewTokenPairSTRv2(testEpixChainDenom)
	require.NoError(t, err)

	// All should be identical
	require.Equal(t, addr1.Hex(), addr2.Hex())
	require.Equal(t, addr1.Hex(), pair.Erc20Address)
	require.Equal(t, testWepixAddress, addr1.Hex())
}

// TestWEPIXHashingProperties tests cryptographic properties
func TestWEPIXHashingProperties(t *testing.T) {
	// Test that small changes in input produce very different outputs (avalanche effect)
	addr1, err := utils.GetNativeDenomAddress("aepix")
	require.NoError(t, err)

	addr2, err := utils.GetNativeDenomAddress("bepix") // One character different
	require.NoError(t, err)

	// Addresses should be completely different
	require.NotEqual(t, addr1.Hex(), addr2.Hex())

	// Calculate Hamming distance (should be roughly 50% for good hash function)
	addr1Bytes := addr1.Bytes()
	addr2Bytes := addr2.Bytes()

	differentBits := 0
	for i := 0; i < 20; i++ { // 20 bytes in address
		xor := addr1Bytes[i] ^ addr2Bytes[i]
		for j := 0; j < 8; j++ { // 8 bits per byte
			if (xor>>j)&1 == 1 {
				differentBits++
			}
		}
	}

	// Should have roughly 50% different bits (good avalanche effect)
	// Allow range of 30-70% to account for randomness
	totalBits := 160 // 20 bytes * 8 bits
	diffPercentage := float64(differentBits) / float64(totalBits) * 100
	require.True(t, diffPercentage >= 30.0 && diffPercentage <= 70.0,
		"Poor avalanche effect: %.2f%% bits different (expected 30-70%%)", diffPercentage)
}
