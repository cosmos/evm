package common

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/types/query"
)

type pageRequestTuple struct {
	Key        []byte
	Offset     uint64
	Limit      uint64
	CountTotal bool
	Reverse    bool
}

func TestPageRequestFromArg(t *testing.T) {
	want := query.PageRequest{Key: []byte{1, 2}, Offset: 3, Limit: 4, CountTotal: true, Reverse: true}

	t.Run("typed PageRequest", func(t *testing.T) {
		got, err := PageRequestFromArg(SharedErrorABI, "validators", 1, want)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("anonymous ABI tuple", func(t *testing.T) {
		got, err := PageRequestFromArg(SharedErrorABI, "validators", 1, pageRequestTuple{
			Key: want.Key, Offset: want.Offset, Limit: want.Limit, CountTotal: want.CountTotal, Reverse: want.Reverse,
		})
		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("invalid tuple returns shared Solidity error", func(t *testing.T) {
		_, err := PageRequestFromArg(SharedErrorABI, "validators", 1, "bad-page")
		var carrier RevertDataCarrier
		require.ErrorAs(t, err, &carrier)

		data := carrier.RevertData()
		require.Equal(t, []byte{0x7a, 0x5b, 0xf5, 0xd2}, data[:4])
		definition := SharedErrorABI.Errors[SolidityErrInvalidPageRequest]
		require.Equal(t, definition.ID[:4], data[:4])
		decoded, unpackErr := definition.Inputs.Unpack(data[4:])
		require.NoError(t, unpackErr)
		require.Equal(t, []interface{}{"validators", big.NewInt(1), "bad-page"}, decoded)
	})
}
