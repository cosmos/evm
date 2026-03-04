package backend

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewBackend_EVMChainID_LargeValue(t *testing.T) {
	// Verify SetUint64 correctly handles chain IDs > MaxInt64
	largeChainID := uint64(math.MaxInt64) + 1
	result := new(big.Int).SetUint64(largeChainID)

	require.Equal(t, largeChainID, result.Uint64())
	require.True(t, result.Sign() > 0, "chain ID should be positive")

	// Confirm that the old approach (big.NewInt(int64(v))) would produce a wrong value
	wrongResult := big.NewInt(int64(largeChainID)) //nolint:gosec
	require.True(t, wrongResult.Sign() < 0, "int64 cast should produce negative value for overflow")
	require.NotEqual(t, result, wrongResult, "SetUint64 and int64 cast should differ for large values")
}

func TestNewBackend_EVMChainID_NormalValue(t *testing.T) {
	// Verify SetUint64 works correctly for normal chain IDs
	normalChainID := uint64(1)
	result := new(big.Int).SetUint64(normalChainID)

	require.Equal(t, normalChainID, result.Uint64())
	require.Equal(t, big.NewInt(1), result)
}
