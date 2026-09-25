package werc20

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	erc20 "github.com/cosmos/evm/precompiles/erc20"
	precompiletest "github.com/cosmos/evm/precompiles/testutil"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

var errSyntheticWERC20Drift = errorsmod.Register("werc20-phase-four-drift", 77, "unstable reason")

func TestWERC20UnmappedReturnsUnmappedRevert(t *testing.T) {
	p := Precompile{Precompile: &erc20.Precompile{ABI: ABI}}

	err := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, WithdrawMethod, errSyntheticWERC20Drift, nil).Err
	carrier := err.(cmn.RevertDataCarrier)
	unmappedDefinition := ABI.Errors[cmn.SolidityErrUnmappedCosmosError]
	require.Equal(t, unmappedDefinition.ID[:4], carrier.RevertData()[:4])

	known := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, WithdrawMethod, banktypes.ErrSendDisabled, nil).Err
	knownDefinition := ABI.Errors[erc20.SolidityErrBankSendDisabled]
	require.Equal(t, knownDefinition.ID[:4], known.(cmn.RevertDataCarrier).RevertData()[:4])
}

func TestWERC20WrappedBankSentinelReturnsConcreteParsedError(t *testing.T) {
	p := Precompile{Precompile: &erc20.Precompile{ABI: ABI}}
	wrapped := errorsmod.Wrap(banktypes.ErrSendDisabled, "changed bank wrapper text")

	err := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, DepositMethod, wrapped, nil).Err
	data := err.(cmn.RevertDataCarrier).RevertData()
	expected := cmn.NewRevertWithSolidityError(ABI, erc20.SolidityErrBankSendDisabled).(cmn.RevertDataCarrier).RevertData()
	require.Equal(t, expected, data)

	var selector [4]byte
	copy(selector[:], data[:4])
	definition, parseErr := ABI.ErrorByID(selector)
	require.NoError(t, parseErr)
	require.Equal(t, erc20.SolidityErrBankSendDisabled, definition.Name)
	_, parseErr = definition.Unpack(data)
	require.NoError(t, parseErr)

	for _, fallback := range []string{
		cmn.SolidityErrMsgServerFailed,
		cmn.SolidityErrQueryFailed,
		cmn.SolidityErrUnmappedCosmosError,
	} {
		fallbackDefinition := ABI.Errors[fallback]
		require.NotEqual(t, fallbackDefinition.ID[:4], data[:4])
	}
}

func TestWERC20BoundaryPreservationAndEquivalence(t *testing.T) {
	p := Precompile{Precompile: &erc20.Precompile{ABI: ABI}}
	t.Run("msg", func(t *testing.T) {
		adapter := func(ctx sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveMsgServerError(p.ABI, "method", err, nil).Err
		}
		precompiletest.TestBoundaryAdapter(t, adapter)
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, "method", true, adapter, errSyntheticWERC20Drift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	})
}
