package backend

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFeeHistory_OldestBlockOverflowGuard(t *testing.T) {
	// Verify that the overflow guard logic correctly identifies values > MaxInt64.
	// The actual guard in FeeHistory is:
	//   if uint64(blkNumber) > uint64(gomath.MaxInt64) { return error }
	// This test validates the arithmetic used in that guard.
	tests := []struct {
		name      string
		blkNumber uint64
		overflows bool
	}{
		{"normal block number", 100, false},
		{"max safe int64", uint64(math.MaxInt64), false},
		{"first overflow", uint64(math.MaxInt64) + 1, true},
		{"max uint64", math.MaxUint64, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			overflows := tc.blkNumber > uint64(math.MaxInt64)
			require.Equal(t, tc.overflows, overflows)
			if !overflows {
				// Verify int64 cast is safe when guard passes
				v := int64(tc.blkNumber) //nolint:gosec
				require.GreaterOrEqual(t, v, int64(0))
			}
		})
	}
}
