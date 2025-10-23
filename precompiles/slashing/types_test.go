package slashing

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	evmaddress "github.com/cosmos/evm/encoding/address"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestParseSigningInfoArgs(t *testing.T) {
	consCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32ConsensusAddrPrefix())
	validAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	expectedConsAddr, err := consCodec.BytesToString(validAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name            string
		args            GetSigningInfoCall
		wantErr         bool
		wantConsAddress string
	}{
		{
			name: "valid address",
			args: GetSigningInfoCall{
				ConsAddress: validAddr,
			},
			wantErr:         false,
			wantConsAddress: expectedConsAddr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSigningInfoArgs(tt.args, consCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, tt.wantConsAddress, got.ConsAddress)
			}
		})
	}
}
