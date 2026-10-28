package gov

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
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

var errSyntheticGovDrift = errorsmod.Register("gov-phase-three-drift", 77, "unstable reason")

func TestTranslateGovRegisteredErrorsDirectAndWrapped(t *testing.T) {
	p := Precompile{ABI: ABI}
	for _, err := range []error{
		govtypes.ErrInvalidVote,
		errorsmod.Wrap(govtypes.ErrInvalidVote, "message changed"),
		fmt.Errorf("standard wrapper: %w", govtypes.ErrInvalidVote),
	} {
		translated := cosmosErrorRegistry.ResolveQueryError(p.ABI, VoteMethod, err, nil).Err
		carrier := translated.(cmn.RevertDataCarrier)
		require.Equal(t, govErrorSelector(SolidityErrGovInvalidVote), carrier.RevertData())
		require.NotEqual(t, govErrorSelector(cmn.SolidityErrMsgServerFailed), carrier.RevertData()[:4])
		require.NotEqual(t, govErrorSelector(cmn.SolidityErrUnmappedCosmosError), carrier.RevertData()[:4])
	}
}

func TestGovUnregisteredAndGRPCFailuresKeepLegacyFallbacks(t *testing.T) {
	p := Precompile{ABI: ABI}

	msgErr := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, DepositMethod, errors.New("infrastructure"), nil).Err
	require.Equal(t, govErrorSelector(cmn.SolidityErrMsgServerFailed), msgErr.(cmn.RevertDataCarrier).RevertData()[:4])

	queryErr := cosmosErrorRegistry.ResolveQueryError(p.ABI, GetProposalMethod, status.Error(codes.NotFound, "proposal missing"), nil).Err
	require.Equal(t, govErrorSelector(cmn.SolidityErrQueryFailed), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
	require.NotEqual(t, govErrorSelector(SolidityErrGovInvalidProposal), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
}

func TestTranslateGovUnmappedReturnsUnmappedRevert(t *testing.T) {
	p := Precompile{ABI: ABI}

	err := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, DepositMethod, errSyntheticGovDrift, nil).Err
	carrier := err.(cmn.RevertDataCarrier)
	require.Equal(t, govErrorSelector(cmn.SolidityErrUnmappedCosmosError), carrier.RevertData()[:4])
}

func govErrorSelector(name string) []byte {
	definition := ABI.Errors[name]
	return definition.ID[:4]
}

func TestGovBoundaryPreservationAndEquivalence(t *testing.T) {
	p := Precompile{ABI: ABI}
	t.Run("query", func(t *testing.T) {
		adapter := func(ctx sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveQueryError(p.ABI, "method", err, nil).Err
		}
		precompiletest.TestBoundaryAdapter(t, adapter)
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, "method", false, adapter, errSyntheticGovDrift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	})
	t.Run("msg", func(t *testing.T) {
		adapter := func(ctx sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveMsgServerError(p.ABI, "method", err, nil).Err
		}
		precompiletest.TestBoundaryAdapter(t, adapter)
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, "method", true, adapter, errSyntheticGovDrift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	})
}
