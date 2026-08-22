package gov

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
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

var errSyntheticGovDrift = errorsmod.Register("gov-phase-three-drift", 77, "unstable reason")

func TestTranslateGovRegisteredErrorsDirectAndWrapped(t *testing.T) {
	p := Precompile{ABI: ABI}
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())
	for _, err := range []error{
		govtypes.ErrInvalidVote,
		errorsmod.Wrap(govtypes.ErrInvalidVote, "message changed"),
		fmt.Errorf("standard wrapper: %w", govtypes.ErrInvalidVote),
	} {
		translated := p.translateGovError(ctx, VoteMethod, err)
		carrier := translated.(cmn.RevertDataCarrier)
		require.Equal(t, govErrorSelector(SolidityErrGovInvalidVote), carrier.RevertData())
		require.NotEqual(t, govErrorSelector(cmn.SolidityErrMsgServerFailed), carrier.RevertData()[:4])
		require.NotEqual(t, govErrorSelector(cmn.SolidityErrUnmappedCosmosError), carrier.RevertData()[:4])
	}
}

func TestGovUnregisteredAndGRPCFailuresKeepLegacyFallbacks(t *testing.T) {
	p := Precompile{ABI: ABI}
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())

	msgErr := p.govMsgError(ctx, DepositMethod, errors.New("infrastructure"))
	require.Equal(t, govErrorSelector(cmn.SolidityErrMsgServerFailed), msgErr.(cmn.RevertDataCarrier).RevertData()[:4])

	queryErr := p.govQueryError(ctx, GetProposalMethod, status.Error(codes.NotFound, "proposal missing"))
	require.Equal(t, govErrorSelector(cmn.SolidityErrQueryFailed), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
	require.NotEqual(t, govErrorSelector(SolidityErrGovInvalidProposal), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
}

func TestTranslateGovUnmappedLogsExactlyOnceWithoutReason(t *testing.T) {
	var output bytes.Buffer
	ctx := sdk.Context{}.WithLogger(log.NewLogger(&output, log.OutputJSONOption()))
	p := Precompile{ABI: ABI}

	err := p.govMsgError(ctx, DepositMethod, errSyntheticGovDrift)
	carrier := err.(cmn.RevertDataCarrier)
	require.Equal(t, govErrorSelector(cmn.SolidityErrUnmappedCosmosError), carrier.RevertData()[:4])

	logs := output.String()
	require.Equal(t, 1, strings.Count(logs, "unmapped registered Cosmos error"))
	require.Contains(t, logs, `"precompile":"gov"`)
	require.Contains(t, logs, `"method":"deposit"`)
	require.Contains(t, logs, `"codespace":"gov-phase-three-drift"`)
	require.Contains(t, logs, `"code":77`)
	require.NotContains(t, logs, "unstable reason")

	_ = p.govMsgError(ctx, DepositMethod, govtypes.ErrInvalidVote)
	require.Equal(t, 1, strings.Count(output.String(), "unmapped registered Cosmos error"), "known mappings must not emit the unmapped signal")
}

func govErrorSelector(name string) []byte {
	definition := ABI.Errors[name]
	return definition.ID[:4]
}
