package slashing

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	precompiletest "github.com/cosmos/evm/precompiles/testutil"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
)

var errSyntheticSlashingDrift = errorsmod.Register("slashing-phase-four-drift", 77, "unstable reason")

func TestTranslateSlashingRegisteredErrorsDirectAndWrapped(t *testing.T) {
	p := Precompile{ABI: ABI}

	for _, returned := range []error{
		slashingtypes.ErrValidatorNotJailed,
		errorsmod.Wrap(slashingtypes.ErrValidatorNotJailed, "changed text"),
		fmt.Errorf("standard wrapper: %w", slashingtypes.ErrValidatorNotJailed),
	} {
		err := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, UnjailMethod, returned, nil).Err
		carrier := err.(cmn.RevertDataCarrier)
		require.Equal(t, slashingErrorSelector(SolidityErrSlashingValidatorNotJailed), carrier.RevertData())
		require.NotEqual(t, slashingErrorSelector(cmn.SolidityErrMsgServerFailed), carrier.RevertData()[:4])
		require.NotEqual(t, slashingErrorSelector(cmn.SolidityErrUnmappedCosmosError), carrier.RevertData()[:4])
	}
}

func TestSlashingUnregisteredFailuresKeepInternalErrors(t *testing.T) {
	p := Precompile{ABI: ABI}
	internal := errors.New("infrastructure failure")

	msgErr := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, UnjailMethod, internal, nil).Err
	require.Equal(t, slashingErrorSelector(cmn.SolidityErrMsgServerFailed), msgErr.(cmn.RevertDataCarrier).RevertData()[:4])

	queryErr := cosmosErrorRegistry.ResolveQueryError(p.ABI, GetParamsMethod, internal, nil).Err
	require.Equal(t, slashingErrorSelector(cmn.SolidityErrQueryFailed), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
}

func TestSlashingUnmappedReturnsUnmappedRevert(t *testing.T) {
	p := Precompile{ABI: ABI}

	err := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, UnjailMethod, errSyntheticSlashingDrift, nil).Err
	require.Equal(t, slashingErrorSelector(cmn.SolidityErrUnmappedCosmosError), err.(cmn.RevertDataCarrier).RevertData()[:4])
}

func slashingErrorSelector(name string) []byte {
	definition := ABI.Errors[name]
	return definition.ID[:4]
}

func TestSlashingBoundaryPreservationAndEquivalence(t *testing.T) {
	p := Precompile{ABI: ABI}
	t.Run("query", func(t *testing.T) {
		adapter := func(ctx sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveQueryError(p.ABI, "method", err, nil).Err
		}
		precompiletest.TestBoundaryAdapter(t, adapter)
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, "method", false, adapter, errSyntheticSlashingDrift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	})
	t.Run("msg", func(t *testing.T) {
		adapter := func(_ sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveMsgServerError(p.ABI, UnjailMethod, err, nil).Err
		}
		precompiletest.TestBoundaryAdapter(t, adapter)
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, UnjailMethod, true, adapter, errSyntheticSlashingDrift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	})
}
