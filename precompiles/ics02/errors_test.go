package ics02

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	precompiletest "github.com/cosmos/evm/precompiles/testutil"
	clienttypes "github.com/cosmos/ibc-go/v11/modules/core/02-client/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var errSyntheticICS02Drift = errorsmod.Register("ics02-phase-three-drift", 77, "unstable reason")

func TestTranslateICS02RegisteredErrorsDirectAndWrapped(t *testing.T) {
	p := Precompile{ABI: ABI}
	tests := []struct {
		err  error
		name string
	}{
		{clienttypes.ErrInvalidClientType, SolidityErrIBCClientInvalidClientType},
		{clienttypes.ErrRouteNotFound, SolidityErrIBCClientRouteNotFound},
		{clienttypes.ErrClientNotActive, SolidityErrIBCClientNotActive},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, returned := range []error{
				tc.err,
				errorsmod.Wrap(tc.err, "message changed"),
				fmt.Errorf("standard wrapper: %w", tc.err),
			} {
				got := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, VerifyMembershipMethod, returned, nil).Err
				require.Equal(t, ics02ErrorSelector(tc.name), got.(cmn.RevertDataCarrier).RevertData())
				assertICS02NotFallback(t, got)
			}
		})
	}
}

func TestICS02UnregisteredKeeperAndQueryFailuresKeepLegacyFallbacks(t *testing.T) {
	p := Precompile{ABI: ABI}
	plain := errors.New("infrastructure")

	msgErr := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, UpdateClientMethod, plain, nil).Err
	require.Equal(t, ics02ErrorSelector(cmn.SolidityErrMsgServerFailed), msgErr.(cmn.RevertDataCarrier).RevertData()[:4])

	queryErr := cosmosErrorRegistry.ResolveQueryError(p.ABI, GetClientStateMethod, plain, nil).Err
	require.Equal(t, ics02ErrorSelector(cmn.SolidityErrQueryFailed), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
}

func TestICS02ValidatedInputMapsPublishedError(t *testing.T) {
	p := Precompile{ABI: ABI}

	mapped := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, UpdateClientMethod, clienttypes.ErrInvalidClientType, nil).Err
	require.Equal(t, ics02ErrorSelector(SolidityErrIBCClientInvalidClientType), mapped.(cmn.RevertDataCarrier).RevertData())
	assertICS02NotFallback(t, mapped)
}

func TestICS02ValidatedInputUnmappedProtocolErrorReturnsUnmappedRevert(t *testing.T) {
	p := Precompile{ABI: ABI}
	returned := fmt.Errorf("validation wrapper with changed message: %w", errSyntheticICS02Drift)
	classification := cmn.TranslateCosmosError(ABI, cosmosErrorRegistry, returned)
	require.Equal(t, cmn.MappingKindUnmapped, classification.Kind)
	key, ok := cmn.ExtractCosmosErrorKey(returned)
	require.True(t, ok)

	err := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, UpdateClientMethod, returned, nil).Err
	expected := classification.Revert
	require.Equal(t, expected.(cmn.RevertDataCarrier).RevertData(), err.(cmn.RevertDataCarrier).RevertData())
	decoded, unpackErr := ABI.Errors[cmn.SolidityErrUnmappedCosmosError].Inputs.Unpack(err.(cmn.RevertDataCarrier).RevertData()[4:])
	require.NoError(t, unpackErr)
	require.Equal(t, []any{key.Codespace, key.Code}, decoded)
}

func TestICS02UnmappedRegisteredErrorReturnsUnmappedRevert(t *testing.T) {
	p := Precompile{ABI: ABI}

	err := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, UpdateClientMethod, errSyntheticICS02Drift, nil).Err
	require.Equal(t, ics02ErrorSelector(cmn.SolidityErrUnmappedCosmosError), err.(cmn.RevertDataCarrier).RevertData()[:4])
}

func assertICS02NotFallback(t *testing.T, err error) {
	t.Helper()
	selector := err.(cmn.RevertDataCarrier).RevertData()[:4]
	require.NotEqual(t, ics02ErrorSelector(cmn.SolidityErrMsgServerFailed), selector)
	require.NotEqual(t, ics02ErrorSelector(cmn.SolidityErrQueryFailed), selector)
	require.NotEqual(t, ics02ErrorSelector(cmn.SolidityErrUnmappedCosmosError), selector)
}

func ics02ErrorSelector(name string) []byte {
	definition := ABI.Errors[name]
	return definition.ID[:4]
}

func TestICS02BoundaryPreservationAndEquivalence(t *testing.T) {
	p := Precompile{ABI: ABI}
	t.Run("query", func(t *testing.T) {
		adapter := func(ctx sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveQueryError(p.ABI, "method", err, nil).Err
		}
		precompiletest.TestBoundaryAdapter(t, adapter)
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, "method", false, adapter, errSyntheticICS02Drift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	})
	t.Run("keeper", func(t *testing.T) {
		adapter := func(ctx sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveMsgServerError(p.ABI, "method", err, nil).Err
		}
		precompiletest.TestBoundaryAdapter(t, adapter)
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, "method", true, adapter, errSyntheticICS02Drift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	})
	t.Run("validated", func(t *testing.T) {
		adapter := func(_ sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveMsgServerError(p.ABI, UpdateClientMethod, err, nil).Err
		}
		precompiletest.TestBoundaryAdapter(t, adapter)
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, UpdateClientMethod, true, adapter, errSyntheticICS02Drift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	})
}
