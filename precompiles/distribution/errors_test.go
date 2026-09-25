package distribution

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cmn "github.com/cosmos/evm/precompiles/common"
	precompiletest "github.com/cosmos/evm/precompiles/testutil"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var errSyntheticDistributionDrift = errorsmod.Register("distribution-phase-three-drift", 77, "unstable reason")

func TestTranslateDistributionRegisteredErrorsDirectAndWrapped(t *testing.T) {
	p := Precompile{ABI: ABI}
	testCases := []struct {
		err      error
		expected string
	}{
		{distributiontypes.ErrEmptyDelegationDistInfo, SolidityErrDistributionEmptyDelegationDistributionInfo},
		{errorsmod.Wrap(distributiontypes.ErrEmptyDelegationDistInfo, "message changed"), SolidityErrDistributionEmptyDelegationDistributionInfo},
		{fmt.Errorf("standard wrapper: %w", distributiontypes.ErrEmptyDelegationDistInfo), SolidityErrDistributionEmptyDelegationDistributionInfo},
		{stakingtypes.ErrNoValidatorFound, SolidityErrDistributionNoValidatorExists},
		{errorsmod.Wrap(stakingtypes.ErrNoValidatorFound, "dependency message changed"), SolidityErrDistributionNoValidatorExists},
		{stakingtypes.ErrNoDelegation, SolidityErrDistributionNoDelegationExists},
		{fmt.Errorf("dependency wrapper: %w", stakingtypes.ErrNoDelegation), SolidityErrDistributionNoDelegationExists},
	}
	for _, tc := range testCases {
		translated := cosmosErrorRegistry.ResolveQueryError(p.ABI, DelegationRewardsMethod, tc.err, nil).Err
		carrier := translated.(cmn.RevertDataCarrier)
		require.Equal(t, distributionErrorSelector(tc.expected), carrier.RevertData())
		require.NotEqual(t, distributionErrorSelector(cmn.SolidityErrQueryFailed), carrier.RevertData()[:4])
		require.NotEqual(t, distributionErrorSelector(cmn.SolidityErrUnmappedCosmosError), carrier.RevertData()[:4])
	}
}

func TestDistributionUnregisteredAndGRPCFailuresKeepLegacyFallbacks(t *testing.T) {
	p := Precompile{ABI: ABI}

	msgErr := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, FundCommunityPoolMethod, errors.New("infrastructure"), nil).Err
	require.Equal(t, distributionErrorSelector(cmn.SolidityErrMsgServerFailed), msgErr.(cmn.RevertDataCarrier).RevertData()[:4])

	queryErr := cosmosErrorRegistry.ResolveQueryError(p.ABI, ValidatorCommissionMethod, status.Error(codes.NotFound, "message changed"), nil).Err
	require.Equal(t, distributionErrorSelector(cmn.SolidityErrQueryFailed), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
	require.NotEqual(t, distributionErrorSelector(SolidityErrDistributionNoValidatorExists), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
}

func TestTranslateDistributionUnmappedReturnsUnmappedRevert(t *testing.T) {
	p := Precompile{ABI: ABI}

	err := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, FundCommunityPoolMethod, errSyntheticDistributionDrift, nil).Err
	carrier := err.(cmn.RevertDataCarrier)
	require.Equal(t, distributionErrorSelector(cmn.SolidityErrUnmappedCosmosError), carrier.RevertData()[:4])
}

func distributionErrorSelector(name string) []byte {
	definition := ABI.Errors[name]
	return definition.ID[:4]
}

func TestDistributionBoundaryPreservation(t *testing.T) {
	p := Precompile{ABI: ABI}
	t.Run("query", func(t *testing.T) {
		precompiletest.TestBoundaryAdapter(t, func(ctx sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveQueryError(p.ABI, "query", err, nil).Err
		})
	})
	t.Run("msg", func(t *testing.T) {
		precompiletest.TestBoundaryAdapter(t, func(ctx sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveMsgServerError(p.ABI, "msg", err, nil).Err
		})
	})
}

func TestDistributionBoundaryEquivalence(t *testing.T) {
	p := Precompile{ABI: ABI}
	for _, msg := range []bool{false, true} {
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, "method", msg, func(ctx sdk.Context, err error) error {
			if msg {
				return cosmosErrorRegistry.ResolveMsgServerError(p.ABI, "method", err, nil).Err
			}
			return cosmosErrorRegistry.ResolveQueryError(p.ABI, "method", err, nil).Err
		}, errSyntheticDistributionDrift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	}
}
