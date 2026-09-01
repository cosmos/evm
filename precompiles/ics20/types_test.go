package ics20

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/testutil"

	"github.com/cosmos/cosmos-sdk/types/query"
)

func TestNewDenomsRequest(t *testing.T) {
	method, ok := ABI.Methods[DenomsMethod]
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
		req, err := NewDenomsRequest(&method, args)

		require.NoError(t, err)
		require.NotNil(t, req)
		require.Equal(t, &pageRequest, req.Pagination)
	})

	t.Run("invalid pagination", func(t *testing.T) {
		req, err := NewDenomsRequest(&method, []interface{}{"bad-pagination"})
		wantErr := cmn.NewRevertWithSolidityError(
			ABI,
			cmn.SolidityErrInvalidPageRequest,
			DenomsMethod,
			big.NewInt(0),
			"bad-pagination",
		)

		testutil.RequireExactError(t, err, wantErr)
		require.Nil(t, req)
	})
}
