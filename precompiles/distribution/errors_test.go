package distribution

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cmn "github.com/cosmos/evm/precompiles/common"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var errSyntheticDistributionDrift = errorsmod.Register("distribution-phase-three-drift", 77, "unstable reason")

func TestTranslateDistributionRegisteredErrorsDirectAndWrapped(t *testing.T) {
	p := Precompile{ABI: ABI}
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())
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
		translated := p.translateDistributionError(ctx, DelegationRewardsMethod, tc.err)
		carrier := translated.(cmn.RevertDataCarrier)
		require.Equal(t, distributionErrorSelector(tc.expected), carrier.RevertData())
		require.NotEqual(t, distributionErrorSelector(cmn.SolidityErrQueryFailed), carrier.RevertData()[:4])
		require.NotEqual(t, distributionErrorSelector(cmn.SolidityErrUnmappedCosmosError), carrier.RevertData()[:4])
	}
}

func TestDistributionUnregisteredAndGRPCFailuresKeepLegacyFallbacks(t *testing.T) {
	p := Precompile{ABI: ABI}
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())

	msgErr := p.distributionMsgError(ctx, FundCommunityPoolMethod, errors.New("infrastructure"))
	require.Equal(t, distributionErrorSelector(cmn.SolidityErrMsgServerFailed), msgErr.(cmn.RevertDataCarrier).RevertData()[:4])

	queryErr := p.distributionQueryError(ctx, ValidatorCommissionMethod, status.Error(codes.NotFound, "message changed"))
	require.Equal(t, distributionErrorSelector(cmn.SolidityErrQueryFailed), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
	require.NotEqual(t, distributionErrorSelector(SolidityErrDistributionNoValidatorExists), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
}

func TestTranslateDistributionUnmappedLogsExactlyOnceWithoutReason(t *testing.T) {
	var output bytes.Buffer
	ctx := sdk.Context{}.WithLogger(log.NewLogger(&output, log.OutputJSONOption()))
	p := Precompile{ABI: ABI}

	err := p.distributionMsgError(ctx, FundCommunityPoolMethod, errSyntheticDistributionDrift)
	carrier := err.(cmn.RevertDataCarrier)
	require.Equal(t, distributionErrorSelector(cmn.SolidityErrUnmappedCosmosError), carrier.RevertData()[:4])

	logs := output.String()
	require.Equal(t, 1, strings.Count(logs, "unmapped registered Cosmos error"))
	require.Contains(t, logs, `"precompile":"distribution"`)
	require.Contains(t, logs, `"method":"fundCommunityPool"`)
	require.Contains(t, logs, `"codespace":"distribution-phase-three-drift"`)
	require.Contains(t, logs, `"code":77`)
	require.NotContains(t, logs, "unstable reason")

	_ = p.distributionMsgError(ctx, FundCommunityPoolMethod, distributiontypes.ErrEmptyDelegationDistInfo)
	require.Equal(t, 1, strings.Count(output.String(), "unmapped registered Cosmos error"), "known mappings must not emit the unmapped signal")
}

func distributionErrorSelector(name string) []byte {
	definition := ABI.Errors[name]
	return definition.ID[:4]
}
