package slashing

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
)

var errSyntheticSlashingDrift = errorsmod.Register("slashing-phase-four-drift", 77, "unstable reason")

func TestTranslateSlashingRegisteredErrorsDirectAndWrapped(t *testing.T) {
	p := Precompile{ABI: ABI}
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())

	for _, returned := range []error{
		slashingtypes.ErrValidatorNotJailed,
		errorsmod.Wrap(slashingtypes.ErrValidatorNotJailed, "changed text"),
		fmt.Errorf("standard wrapper: %w", slashingtypes.ErrValidatorNotJailed),
	} {
		err := p.slashingMsgError(ctx, returned)
		carrier := err.(cmn.RevertDataCarrier)
		require.Equal(t, slashingErrorSelector(SolidityErrSlashingValidatorNotJailed), carrier.RevertData())
		require.NotEqual(t, slashingErrorSelector(cmn.SolidityErrMsgServerFailed), carrier.RevertData()[:4])
		require.NotEqual(t, slashingErrorSelector(cmn.SolidityErrUnmappedCosmosError), carrier.RevertData()[:4])
	}
}

func TestSlashingUnregisteredFailuresKeepInternalErrors(t *testing.T) {
	p := Precompile{ABI: ABI}
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())
	internal := errors.New("infrastructure failure")

	msgErr := p.slashingMsgError(ctx, internal)
	require.Equal(t, slashingErrorSelector(cmn.SolidityErrMsgServerFailed), msgErr.(cmn.RevertDataCarrier).RevertData()[:4])

	queryErr := p.slashingQueryError(ctx, GetParamsMethod, internal)
	require.Equal(t, slashingErrorSelector(cmn.SolidityErrQueryFailed), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
}

func TestSlashingUnmappedLogsExactlyOnceWithoutReason(t *testing.T) {
	var output bytes.Buffer
	ctx := sdk.Context{}.WithLogger(log.NewLogger(&output, log.OutputJSONOption()))
	p := Precompile{ABI: ABI}

	err := p.slashingMsgError(ctx, errSyntheticSlashingDrift)
	require.Equal(t, slashingErrorSelector(cmn.SolidityErrUnmappedCosmosError), err.(cmn.RevertDataCarrier).RevertData()[:4])

	logs := output.String()
	require.Equal(t, 1, strings.Count(logs, "unmapped registered Cosmos error"))
	require.Contains(t, logs, `"precompile":"slashing"`)
	require.Contains(t, logs, `"method":"unjail"`)
	require.Contains(t, logs, `"codespace":"slashing-phase-four-drift"`)
	require.Contains(t, logs, `"code":77`)
	require.NotContains(t, logs, "unstable reason")

	_ = p.slashingMsgError(ctx, slashingtypes.ErrValidatorNotJailed)
	require.Equal(t, 1, strings.Count(output.String(), "unmapped registered Cosmos error"), "known mappings must not emit the unmapped signal")
}

func slashingErrorSelector(name string) []byte {
	definition := ABI.Errors[name]
	return definition.ID[:4]
}
