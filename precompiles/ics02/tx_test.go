package ics02

import (
	"testing"

	"github.com/stretchr/testify/require"

	clienttypes "github.com/cosmos/ibc-go/v11/modules/core/02-client/types"
	host "github.com/cosmos/ibc-go/v11/modules/core/24-host"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
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

// TestVerifyInvalidMerklePath checks that verifyMembership and
// verifyNonMembership return an error, instead of panicking, for paths with an
// empty or oversized key. The localhost client reads such a key from the store,
// which panics.
func TestVerifyInvalidMerklePath(t *testing.T) {
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	// The client keeper is nil on purpose: the call must fail before reaching it.
	p := NewPrecompile(cdc, nil)

	proofHeight := struct {
		RevisionNumber uint64
		RevisionHeight uint64
	}{RevisionNumber: 0, RevisionHeight: 1}

	testCases := []struct {
		name string
		path [][]byte
	}{
		{name: "empty path", path: [][]byte{}},
		{name: "empty prefix", path: [][]byte{{}, []byte("clients/07-tendermint-0/clientState")}},
		{name: "empty key", path: [][]byte{[]byte("ibc"), {}}},
		{name: "oversized key", path: [][]byte{[]byte("ibc"), make([]byte, storetypes.MaxKeyLength+1)}},
	}

	for _, tc := range testCases {
		t.Run(VerifyMembershipMethod+" "+tc.name, func(t *testing.T) {
			method := p.Methods[VerifyMembershipMethod]
			in, err := p.Pack(VerifyMembershipMethod, "09-localhost", []byte{0x01}, proofHeight, tc.path, []byte{0x01})
			require.NoError(t, err)
			args, err := method.Inputs.Unpack(in[4:])
			require.NoError(t, err)

			var (
				bz      []byte
				callErr error
			)
			require.NotPanics(t, func() {
				bz, callErr = p.VerifyMembership(sdk.Context{}, nil, nil, &method, args)
			})
			require.ErrorIs(t, callErr, host.ErrInvalidPath)
			require.Nil(t, bz)
		})

		t.Run(VerifyNonMembershipMethod+" "+tc.name, func(t *testing.T) {
			method := p.Methods[VerifyNonMembershipMethod]
			in, err := p.Pack(VerifyNonMembershipMethod, "09-localhost", []byte{0x01}, proofHeight, tc.path)
			require.NoError(t, err)
			args, err := method.Inputs.Unpack(in[4:])
			require.NoError(t, err)

			var (
				bz      []byte
				callErr error
			)
			require.NotPanics(t, func() {
				bz, callErr = p.VerifyNonMembership(sdk.Context{}, nil, nil, &method, args)
			})
			require.ErrorIs(t, callErr, host.ErrInvalidPath)
			require.Nil(t, bz)
		})
	}
}
