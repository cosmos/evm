package slashing

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	evmaddress "github.com/cosmos/evm/encoding/address"
	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/testutil"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
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
		wantErrObj      error
		wantConsAddress string
	}{
		{
			name:            "valid address",
			args:            []any{validAddr},
			wantErr:         false,
			wantConsAddress: expectedConsAddr,
		},
		{
			name:       "no arguments",
			args:       []any{},
			wantErr:    true,
			wantErrObj: cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(1), big.NewInt(0)),
		},
		{
			name:       "too many arguments",
			args:       []any{validAddr, "extra"},
			wantErr:    true,
			wantErrObj: cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(1), big.NewInt(2)),
		},
		{
			name:       "invalid type - string instead of address",
			args:       []any{"not-an-address"},
			wantErr:    true,
			wantErrObj: cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, "not-an-address"),
		},
		{
			name:       "invalid type - nil",
			args:       []any{nil},
			wantErr:    true,
			wantErrObj: cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, "<nil>"),
		},
		{
			name:       "empty address",
			args:       []any{common.Address{}},
			wantErr:    true,
			wantErrObj: cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, common.Address{}.String()),
		},
		{
			name:       "invalid type - integer",
			args:       []any{12345},
			wantErr:    true,
			wantErrObj: cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, "12345"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSigningInfoArgs(tt.args, consCodec)

			if tt.wantErr {
				testutil.RequireExactError(t, err, tt.wantErrObj)
				require.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, tt.wantConsAddress, got.ConsAddress)
			}
		})
	}
}

func TestParseSigningInfosArgs(t *testing.T) {
	method, ok := ABI.Methods[GetSigningInfosMethod]
	require.True(t, ok)

	pageRequest := query.PageRequest{
		Key:        []byte{1, 2, 3},
		Offset:     4,
		Limit:      5,
		CountTotal: true,
		Reverse:    true,
	}
	packed, err := method.Inputs.Pack(pageRequest)
	require.NoError(t, err)
	args, err := method.Inputs.Unpack(packed)
	require.NoError(t, err)

	t.Run("valid pagination", func(t *testing.T) {
		req, err := ParseSigningInfosArgs(&method, args)

		require.NoError(t, err)
		require.NotNil(t, req)
		require.Equal(t, &pageRequest, req.Pagination)
	})

	t.Run("invalid pagination", func(t *testing.T) {
		req, err := ParseSigningInfosArgs(&method, []interface{}{"bad-pagination"})
		wantErr := cmn.NewRevertWithSolidityError(
			ABI,
			cmn.SolidityErrInvalidPageRequest,
			GetSigningInfosMethod,
			big.NewInt(0),
			"bad-pagination",
		)

		testutil.RequireExactError(t, err, wantErr)
		require.Nil(t, req)
	})
}
