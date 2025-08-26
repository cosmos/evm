package types_test

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/utils"
)

// TestWEPIXPropertyBased runs fast property-based tests
func TestWEPIXPropertyBased(t *testing.T) {
	// Seed random number generator for reproducible tests
	rand.Seed(time.Now().UnixNano())

	t.Run("deterministic_property", func(t *testing.T) {
		testDeterministicProperty(t)
	})

	t.Run("collision_resistance", func(t *testing.T) {
		testCollisionResistance(t)
	})

	t.Run("valid_address_format", func(t *testing.T) {
		testValidAddressFormat(t)
	})

	t.Run("edge_cases", func(t *testing.T) {
		testEdgeCases(t)
	})

	t.Run("performance", func(t *testing.T) {
		testPerformance(t)
	})
}

func testDeterministicProperty(t *testing.T) {
	testCases := generateRandomDenoms(100)

	for _, denom := range testCases {
		if !isValidDenom(denom) {
			continue
		}

		// Generate address multiple times
		addr1, err1 := utils.GetNativeDenomAddress(denom)
		addr2, err2 := utils.GetNativeDenomAddress(denom)
		addr3, err3 := utils.GetNativeDenomAddress(denom)

		// All should succeed or all should fail
		if err1 != nil || err2 != nil || err3 != nil {
			// If any fails, all should fail consistently
			require.Error(t, err1, "Inconsistent error for denom: %s", denom)
			require.Error(t, err2, "Inconsistent error for denom: %s", denom)
			require.Error(t, err3, "Inconsistent error for denom: %s", denom)
			continue
		}

		// All addresses should be identical
		require.Equal(t, addr1.Hex(), addr2.Hex(), "Non-deterministic address for denom: %s", denom)
		require.Equal(t, addr1.Hex(), addr3.Hex(), "Non-deterministic address for denom: %s", denom)
	}
}

func testCollisionResistance(t *testing.T) {
	addresses := make(map[string]string)
	testCases := generateRandomDenoms(1000)

	for _, denom := range testCases {
		if !isValidDenom(denom) {
			continue
		}

		addr, err := utils.GetNativeDenomAddress(denom)
		if err != nil {
			continue
		}

		addrStr := addr.Hex()

		// Check for collision
		if existingDenom, exists := addresses[addrStr]; exists {
			t.Errorf("Address collision detected! Denom %q and %q both generate address %s",
				denom, existingDenom, addrStr)
		}

		addresses[addrStr] = denom
	}

	t.Logf("Tested %d unique addresses for collision resistance", len(addresses))
}

func testValidAddressFormat(t *testing.T) {
	testCases := generateRandomDenoms(200)

	for _, denom := range testCases {
		if !isValidDenom(denom) {
			continue
		}

		addr, err := utils.GetNativeDenomAddress(denom)
		if err != nil {
			continue
		}

		addrStr := addr.Hex()

		// Validate address format
		require.Len(t, addrStr, 42, "Invalid address length for denom %s: %s", denom, addrStr)
		require.True(t, strings.HasPrefix(addrStr, "0x"), "Address should start with 0x for denom %s: %s", denom, addrStr)
		require.NotEqual(t, "0x0000000000000000000000000000000000000000", addrStr, "Address should not be zero for denom %s", denom)

		// Validate hex characters
		for i, char := range addrStr[2:] { // Skip "0x"
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
				t.Errorf("Invalid hex character at position %d in address %s for denom %s", i+2, addrStr, denom)
			}
		}
	}
}

func testEdgeCases(t *testing.T) {
	edgeCases := []struct {
		denom     string
		shouldErr bool
		reason    string
	}{
		{"", true, "empty string"},
		{"   ", true, "whitespace only"},
		{"a", false, "single character"},
		{"aepix", false, "normal denom"},
		{strings.Repeat("a", 100), false, "long denom"},
		{strings.Repeat("a", 1000), false, "very long denom"},
		{"factory/osmo1234567890/token", false, "factory denom"},
		{"gravity0x1234567890abcdef", false, "gravity denom"},
		{"ibc/ABC123", true, "IBC denom should use different function"},
	}

	for _, tc := range edgeCases {
		t.Run(fmt.Sprintf("edge_case_%s", tc.reason), func(t *testing.T) {
			addr, err := utils.GetNativeDenomAddress(tc.denom)

			if tc.shouldErr {
				require.Error(t, err, "Expected error for %s: %s", tc.reason, tc.denom)
			} else {
				require.NoError(t, err, "Unexpected error for %s: %s", tc.reason, tc.denom)
				require.NotEqual(t, "0x0000000000000000000000000000000000000000", addr.Hex())
			}
		})
	}
}

func testPerformance(t *testing.T) {
	// Test that address generation is fast
	denom := "aepix"
	iterations := 10000

	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := utils.GetNativeDenomAddress(denom)
		require.NoError(t, err)
	}
	duration := time.Since(start)

	avgTime := duration / time.Duration(iterations)
	t.Logf("Average address generation time: %v", avgTime)

	// Should be very fast (less than 1ms per operation)
	require.Less(t, avgTime, time.Millisecond, "Address generation too slow: %v", avgTime)
}

// Helper functions

func generateRandomDenoms(count int) []string {
	denoms := make([]string, count)

	// Include some known good denoms
	knownDenoms := []string{"aepix", "aatom", "stake", "uosmo", "ujuno", "uscrt"}
	for i := 0; i < len(knownDenoms) && i < count; i++ {
		denoms[i] = knownDenoms[i]
	}

	// Generate random denoms
	for i := len(knownDenoms); i < count; i++ {
		denoms[i] = generateRandomDenom()
	}

	return denoms
}

func generateRandomDenom() string {
	// Generate random denom with various patterns
	patterns := []func() string{
		func() string { return randomString(5, 20) },                                           // Simple random string
		func() string { return "a" + randomString(4, 15) },                                     // Cosmos-style (a prefix)
		func() string { return "u" + randomString(4, 15) },                                     // Micro denomination
		func() string { return "factory/" + randomString(10, 20) + "/" + randomString(5, 10) }, // Factory token
		func() string { return "gravity0x" + randomHex(20) },                                   // Gravity bridge
	}

	pattern := patterns[rand.Intn(len(patterns))]
	return pattern()
}

func randomString(minLen, maxLen int) string {
	length := minLen + rand.Intn(maxLen-minLen+1)
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

func randomHex(length int) string {
	chars := "0123456789abcdef"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

func isValidDenom(denom string) bool {
	// Quick validation to skip obviously invalid denoms
	if len(denom) == 0 || len(denom) > 200 {
		return false
	}

	// Skip IBC denoms (they use different logic)
	if strings.HasPrefix(denom, "ibc/") {
		return false
	}

	// Skip denoms with non-printable characters
	for _, r := range denom {
		if r < 32 || r > 126 {
			return false
		}
	}

	return true
}
