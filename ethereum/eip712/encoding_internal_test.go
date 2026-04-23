package eip712

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseChainID(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{name: "canonical (evmd default)", input: "cosmos_262144-1", want: 262144},
		{name: "canonical (ExampleChainID shape)", input: "cosmos_9001-1", want: 9001},
		{name: "canonical with large EVM id", input: "interval_1230263908-1", want: 1230263908},
		{name: "canonical with alphanumeric name", input: "evmos2_9001-42", want: 9001},
		{name: "bare decimal (PR #918 repro)", input: "1230263908", want: 1230263908},

		// Inputs below must error so callers fall back to global eip155ChainID.
		{name: "name-revision, no EVM id", input: "cosmos-1", wantErr: true},
		{name: "classic cosmos chain id", input: "cosmoshub-4", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "missing revision", input: "cosmos_9001", wantErr: true},
		{name: "EVM id leading zero", input: "cosmos_0-1", wantErr: true},
		{name: "revision leading zero", input: "cosmos_9001-0", wantErr: true},
		{name: "name must start with letter", input: "1cosmos_9001-1", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseChainID(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
