package ics02

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	clienttypes "github.com/cosmos/ibc-go/v11/modules/core/02-client/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var errSyntheticICS02Drift = errorsmod.Register("ics02-phase-three-drift", 77, "unstable reason")

func TestTranslateICS02RegisteredErrorsDirectAndWrapped(t *testing.T) {
	p := Precompile{ABI: ABI}
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())
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
				got := p.ics02KeeperError(ctx, VerifyMembershipMethod, returned)
				require.Equal(t, ics02ErrorSelector(tc.name), got.(cmn.RevertDataCarrier).RevertData())
				assertICS02NotFallback(t, got)
			}
		})
	}
}

func TestICS02UnregisteredKeeperAndQueryFailuresKeepLegacyFallbacks(t *testing.T) {
	p := Precompile{ABI: ABI}
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())
	plain := errors.New("infrastructure")

	msgErr := p.ics02KeeperError(ctx, UpdateClientMethod, plain)
	require.Equal(t, ics02ErrorSelector(cmn.SolidityErrMsgServerFailed), msgErr.(cmn.RevertDataCarrier).RevertData()[:4])

	queryErr := p.ics02QueryError(ctx, GetClientStateMethod, plain)
	require.Equal(t, ics02ErrorSelector(cmn.SolidityErrQueryFailed), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
}

func TestICS02ValidatedInputMapsPublishedError(t *testing.T) {
	p := Precompile{ABI: ABI}
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())

	mapped := p.ics02ValidatedInputError(ctx, clienttypes.ErrInvalidClientType)
	require.Equal(t, ics02ErrorSelector(SolidityErrIBCClientInvalidClientType), mapped.(cmn.RevertDataCarrier).RevertData())
	assertICS02NotFallback(t, mapped)
}

func TestICS02ValidatedInputUnmappedProtocolErrorLogsOnceAndReturnsUnmappedRevert(t *testing.T) {
	var output bytes.Buffer
	ctx := sdk.Context{}.WithLogger(log.NewLogger(&output, log.OutputJSONOption()))
	p := Precompile{ABI: ABI}
	returned := fmt.Errorf("validation wrapper with changed message: %w", errSyntheticICS02Drift)
	classification := cmn.TranslateCosmosError(ABI, cosmosErrorRegistry, returned)
	require.Equal(t, cmn.MappingKindUnmapped, classification.Kind)
	key, ok := cmn.ExtractCosmosErrorKey(returned)
	require.True(t, ok)

	err := p.ics02ValidatedInputError(ctx, returned)
	expected := classification.Revert
	require.Equal(t, expected.(cmn.RevertDataCarrier).RevertData(), err.(cmn.RevertDataCarrier).RevertData())

	logs := output.String()
	require.Equal(t, 1, strings.Count(logs, "unmapped registered Cosmos error"))
	require.Contains(t, logs, `"precompile":"ics02"`)
	require.Contains(t, logs, `"method":"updateClient"`)
	require.Contains(t, logs, fmt.Sprintf(`"codespace":"%s"`, key.Codespace))
	require.Contains(t, logs, fmt.Sprintf(`"code":%d`, key.Code))
	require.NotContains(t, logs, "unstable reason")
	require.NotContains(t, logs, "validation wrapper with changed message")

	_ = p.ics02ValidatedInputError(ctx, clienttypes.ErrInvalidClientType)
	require.Equal(t, 1, strings.Count(output.String(), "unmapped registered Cosmos error"), "known mappings must not emit the unmapped signal")
}

func TestICS02UnmappedRegisteredErrorLogsOnceWithoutReason(t *testing.T) {
	var output bytes.Buffer
	ctx := sdk.Context{}.WithLogger(log.NewLogger(&output, log.OutputJSONOption()))
	p := Precompile{ABI: ABI}

	err := p.ics02KeeperError(ctx, UpdateClientMethod, errSyntheticICS02Drift)
	require.Equal(t, ics02ErrorSelector(cmn.SolidityErrUnmappedCosmosError), err.(cmn.RevertDataCarrier).RevertData()[:4])

	logs := output.String()
	require.Equal(t, 1, strings.Count(logs, "unmapped registered Cosmos error"))
	require.Contains(t, logs, `"precompile":"ics02"`)
	require.Contains(t, logs, `"method":"updateClient"`)
	require.Contains(t, logs, `"codespace":"ics02-phase-three-drift"`)
	require.Contains(t, logs, `"code":77`)
	require.NotContains(t, logs, "unstable reason")

	_ = p.ics02KeeperError(ctx, UpdateClientMethod, clienttypes.ErrInvalidClientType)
	require.Equal(t, 1, strings.Count(output.String(), "unmapped registered Cosmos error"), "known mappings must not emit the unmapped signal")
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
