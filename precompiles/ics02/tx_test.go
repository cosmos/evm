package ics02

import (
	"testing"

	"github.com/stretchr/testify/require"

	clienttypes "github.com/cosmos/ibc-go/v11/modules/core/02-client/types"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TestUpdateClientEmptyClientMessage checks that updateClient returns an error,
// instead of panicking, when the client message decodes to nil. This happens
// for empty bytes and for an Any with an empty type URL.
func TestUpdateClientEmptyClientMessage(t *testing.T) {
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	// The client keeper is nil on purpose: the call must fail before reaching it.
	p := NewPrecompile(cdc, nil)

	emptyTypeURL, err := (&codectypes.Any{Value: []byte{0x01, 0x02}}).Marshal()
	require.NoError(t, err)

	testCases := []struct {
		name     string
		updateBz []byte
	}{
		{name: "empty bytes", updateBz: []byte{}},
		{name: "any with empty type url", updateBz: emptyTypeURL},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				bz      []byte
				callErr error
			)
			require.NotPanics(t, func() {
				bz, callErr = p.UpdateClient(sdk.Context{}, nil, nil, nil, []interface{}{"07-tendermint-0", tc.updateBz})
			})
			require.ErrorIs(t, callErr, clienttypes.ErrInvalidClientType)
			require.Nil(t, bz)
		})
	}
}
