package ics02

import (
	"testing"

	"github.com/stretchr/testify/require"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
)

// TestRequiredGas guards against regression of the zero-gas DoS bypass where
// the precompile constructor zeroed KvGasConfig, causing RequiredGas to
// return 0 for every method regardless of calldata size.
func TestRequiredGas(t *testing.T) {
	p := NewPrecompile(nil, nil)
	kv := storetypes.KVGasConfig()

	height := struct {
		RevisionNumber uint64
		RevisionHeight uint64
	}{RevisionNumber: 1, RevisionHeight: 100}

	tests := []struct {
		name    string
		pack    func(t *testing.T) []byte
		isWrite bool
	}{
		{
			name: UpdateClientMethod,
			pack: func(t *testing.T) []byte {
				t.Helper()
				in, err := p.Pack(UpdateClientMethod, "07-tendermint-0", []byte{0x01, 0x02, 0x03})
				require.NoError(t, err)
				return in
			},
			isWrite: true,
		},
		{
			name: VerifyMembershipMethod,
			pack: func(t *testing.T) []byte {
				t.Helper()
				in, err := p.Pack(
					VerifyMembershipMethod,
					"07-tendermint-0",
					[]byte{0xaa, 0xbb},
					height,
					[][]byte{[]byte("ibc"), []byte("clients/07-tendermint-0/clientState")},
					[]byte("value"),
				)
				require.NoError(t, err)
				return in
			},
			isWrite: true,
		},
		{
			name: VerifyNonMembershipMethod,
			pack: func(t *testing.T) []byte {
				t.Helper()
				in, err := p.Pack(
					VerifyNonMembershipMethod,
					"07-tendermint-0",
					[]byte{0xaa, 0xbb},
					height,
					[][]byte{[]byte("ibc"), []byte("clients/07-tendermint-0/clientState")},
				)
				require.NoError(t, err)
				return in
			},
			isWrite: true,
		},
		{
			name: GetClientStateMethod,
			pack: func(t *testing.T) []byte {
				t.Helper()
				in, err := p.Pack(GetClientStateMethod, "07-tendermint-0")
				require.NoError(t, err)
				return in
			},
			isWrite: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := tc.pack(t)

			var expected uint64
			if tc.isWrite {
				expected = kv.WriteCostFlat + kv.WriteCostPerByte*uint64(len(input))
			} else {
				expected = kv.ReadCostFlat + kv.ReadCostPerByte*uint64(len(input))
			}

			gas := p.RequiredGas(input)
			require.Equal(t, expected, gas, "RequiredGas must match the SDK KV-cost formula")
			require.Greater(t, gas, uint64(0), "RequiredGas must be non-zero (zero-gas DoS regression)")
		})
	}

	t.Run("short input returns 0 without panic", func(t *testing.T) {
		require.Equal(t, uint64(0), p.RequiredGas(nil))
		require.Equal(t, uint64(0), p.RequiredGas([]byte{0x01, 0x02, 0x03}))
	})

	t.Run("unknown method ID returns 0", func(t *testing.T) {
		require.Equal(t, uint64(0), p.RequiredGas([]byte{0xde, 0xad, 0xbe, 0xef}))
	})
}
