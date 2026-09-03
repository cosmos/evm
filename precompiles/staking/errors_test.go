package staking

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
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var errSyntheticStakingDrift = errorsmod.Register("staking-phase-two-drift", 77, "unstable reason")

func TestTranslateStakingRegisteredErrorsDirectAndWrapped(t *testing.T) {
	p := Precompile{ABI: ABI}
	for _, err := range []error{
		stakingtypes.ErrNoValidatorFound,
		errorsmod.Wrap(stakingtypes.ErrNoValidatorFound, "message changed"),
		fmt.Errorf("standard wrapper: %w", stakingtypes.ErrNoValidatorFound),
	} {
		translated := cosmosErrorRegistry.ResolveQueryError(p.ABI, DelegateMethod, err, nil).Err
		carrier := translated.(cmn.RevertDataCarrier)
		require.Equal(t, stakingErrorSelector(SolidityErrStakingValidatorNotFound), carrier.RevertData()[:4])
		require.NotEqual(t, stakingErrorSelector(cmn.SolidityErrMsgServerFailed), carrier.RevertData()[:4])
		require.NotEqual(t, stakingErrorSelector(cmn.SolidityErrUnmappedCosmosError), carrier.RevertData()[:4])
	}
}

func TestStakingErrorMappingsReturnsCopyWithoutChangingRuntimeTranslation(t *testing.T) {
	mappings := ErrorMappings()
	require.NotEmpty(t, mappings)
	mappings[0].Key = cmn.NewCosmosErrorKey(stakingtypes.ErrNoDelegation)
	mappings[0].SolidityError = SolidityErrStakingNoDelegation

	fresh := ErrorMappings()
	require.Equal(t, cmn.NewCosmosErrorKey(stakingtypes.ErrNoValidatorFound), fresh[0].Key)
	require.Equal(t, SolidityErrStakingValidatorNotFound, fresh[0].SolidityError)

	p := Precompile{ABI: ABI}
	translated := cosmosErrorRegistry.ResolveQueryError(p.ABI, DelegateMethod, stakingtypes.ErrNoValidatorFound, nil).Err
	require.Equal(t, stakingErrorSelector(SolidityErrStakingValidatorNotFound), translated.(cmn.RevertDataCarrier).RevertData()[:4])
}

func TestTranslateStakingUnmappedReturnsUnmappedRevert(t *testing.T) {
	p := Precompile{ABI: ABI}

	err := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, DelegateMethod, errSyntheticStakingDrift, nil).Err
	carrier := err.(cmn.RevertDataCarrier)
	require.Equal(t, stakingErrorSelector(cmn.SolidityErrUnmappedCosmosError), carrier.RevertData()[:4])
}

func stakingErrorSelector(name string) []byte {
	definition := ABI.Errors[name]
	return definition.ID[:4]
}

func TestStakingBoundaryPreservationAndEquivalence(t *testing.T) {
	p := Precompile{ABI: ABI}
	t.Run("query", func(t *testing.T) {
		adapter := func(ctx sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveQueryError(p.ABI, "method", err, nil).Err
		}
		precompiletest.TestBoundaryAdapter(t, adapter)
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, "method", false, adapter, errSyntheticStakingDrift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	})
	t.Run("msg", func(t *testing.T) {
		adapter := func(ctx sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveMsgServerError(p.ABI, CancelUnbondingDelegationMethod, err, nil).Err
		}
		precompiletest.TestBoundaryAdapter(t, adapter)
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, CancelUnbondingDelegationMethod, true, adapter, errSyntheticStakingDrift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	})
}
