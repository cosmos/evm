package slashing

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	evmaddress "github.com/cosmos/evm/encoding/address"
	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestParseSigningInfoArgs(t *testing.T) {
	consCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32ConsensusAddrPrefix())
	validAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	expectedConsAddr, err := consCodec.BytesToString(validAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name            string
		args            []any
		wantErr         bool
		errMsg          string
		wantConsAddress string
	}{
		{
			name:            "valid address",
			args:            []any{validAddr},
			wantErr:         false,
			wantConsAddress: expectedConsAddr,
		},
		{
			name:    "no arguments",
			args:    []any{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 1, 0),
		},
		{
			name:    "too many arguments",
			args:    []any{validAddr, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 1, 2),
		},
		{
			name:    "invalid type - string instead of address",
			args:    []any{"not-an-address"},
			wantErr: true,
			errMsg:  "invalid consensus address",
		},
		{
			name:    "invalid type - nil",
			args:    []any{nil},
			wantErr: true,
			errMsg:  "invalid consensus address",
		},
		{
			name:    "empty address",
			args:    []any{common.Address{}},
			wantErr: true,
			errMsg:  "invalid consensus address",
		},
		{
			name:    "invalid type - integer",
			args:    []any{12345},
			wantErr: true,
			errMsg:  "invalid consensus address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSigningInfoArgs(tt.args, consCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, tt.wantConsAddress, got.ConsAddress)
			}
		})
	}
}
