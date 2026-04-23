package eip712

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseChainID exercises `parseChainID` against the chain-id shapes
// that actually appear in this repo and in real cosmos-evm deployments.
// These formats are linked to the fix for https://github.com/cosmos/evm/pull/918
// so we need to be explicit about which ones recover a numeric EVM chain id
// and which ones legitimately fall back to the configured global.
func TestParseChainID(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{
			name:  "canonical cosmos-evm chain id (evmd default)",
			input: "cosmos_262144-1",
			want:  262144,
		},
		{
			name:  "canonical cosmos-evm chain id (18-decimal example)",
			input: "cosmos_9001-1",
			want:  9001,
		},
		{
			name:  "canonical chain id with larger EVM chain id",
			input: "interval_1230263908-1",
			want:  1230263908,
		},
		{
			name:  "canonical chain id with alphanumeric name",
			input: "evmos2_9001-42",
			want:  9001,
		},
		{
			name:  "bare decimal chain id (PR #918 reproduction)",
			input: "1230263908",
			want:  1230263908,
		},
		{
			name:    "name-revision without EVM chain id (ExampleChainID)",
			input:   "cosmos-1",
			wantErr: true,
		},
		{
			name:    "classic cosmos chain id",
			input:   "cosmoshub-4",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "name-only, no revision",
			input:   "cosmos_9001",
			wantErr: true,
		},
		{
			name:    "EVM chain id cannot start with 0",
			input:   "cosmos_0-1",
			wantErr: true,
		},
		{
			name:    "revision cannot start with 0",
			input:   "cosmos_9001-0",
			wantErr: true,
		},
		{
			name:    "name must start with a letter",
			input:   "1cosmos_9001-1",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseChainID(tc.input)
			if tc.wantErr {
				require.Error(t, err, "expected parseChainID(%q) to fail so callers fall back to the global eip155ChainID", tc.input)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
