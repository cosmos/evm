package mempool

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/mempool/internal/reaplist"

	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// TestResolveReapListCap verifies the wiring math for the reap list cap.
// Workstream A (STACK-2670): the cap must be the sum of the cosmos pool's
// resolved cap and the legacypool's pending+queued slots, with the special
// cases that an unbounded cosmos pool yields an unbounded reap list and a
// configured-to-zero cosmos pool resolves to the SDK's DefaultMaxTx first.
func TestResolveReapListCap(t *testing.T) {
	// Snapshot the SDK default so the test does not depend on its current
	// numeric value (it has flipped between -1 and 0 across SDK versions).
	sdkDefault := sdkmempool.DefaultMaxTx

	tests := []struct {
		name        string
		cosmosMaxTx int
		globalSlots uint64
		globalQueue uint64
		expectedCap int
		expectUnbnd bool
	}{
		{
			name:        "unbounded cosmos pool -> unbounded reap list",
			cosmosMaxTx: -1,
			globalSlots: 4096,
			globalQueue: 1024,
			expectUnbnd: true,
		},
		{
			name:        "zero cosmos pool resolves to sdk default",
			cosmosMaxTx: 0,
			globalSlots: 4096,
			globalQueue: 1024,
			// behavior depends on whether sdkDefault is -1 (unbounded) or > 0.
			expectedCap: func() int {
				if sdkDefault <= 0 {
					return 0 // unbounded sentinel
				}
				return sdkDefault + 4096 + 1024
			}(),
			expectUnbnd: sdkDefault <= 0,
		},
		{
			name:        "bounded cosmos pool -> sum of all three",
			cosmosMaxTx: 1000,
			globalSlots: 4096,
			globalQueue: 1024,
			expectedCap: 1000 + 4096 + 1024,
		},
		{
			name:        "single slot pool",
			cosmosMaxTx: 1,
			globalSlots: 1,
			globalQueue: 1,
			expectedCap: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveReapListCap(tc.cosmosMaxTx, tc.globalSlots, tc.globalQueue)
			if tc.expectUnbnd {
				require.Equal(t, reaplist.Unbounded, got)
				return
			}
			require.Equal(t, tc.expectedCap, got)
		})
	}
}
