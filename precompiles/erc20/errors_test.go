package erc20

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	precompiletest "github.com/cosmos/evm/precompiles/testutil"
	erc20types "github.com/cosmos/evm/x/erc20/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

var errSyntheticERC20Drift = errorsmod.Register("erc20-phase-three-drift", 77, "unstable reason")

func TestTranslateERC20RegisteredErrorsDirectAndWrapped(t *testing.T) {
	p := Precompile{ABI: ABI}
	tests := []struct {
		name          string
		sentinel      error
		solidityError string
		translate     func(error) error
	}{
		{
			name: "bank send disabled", sentinel: banktypes.ErrSendDisabled, solidityError: SolidityErrBankSendDisabled,
			translate: func(err error) error {
				return cosmosErrorRegistry.ResolveMsgServerError(p.ABI, TransferMethod, err, nil).Err
			},
		},
		{
			name: "token pair not found", sentinel: erc20types.ErrTokenPairNotFound, solidityError: SolidityErrERC20TokenPairNotFound,
			translate: func(err error) error {
				return cosmosErrorRegistry.ResolveQueryError(p.ABI, ApproveMethod, err, nil).Err
			},
		},
		{
			name: "token pair disabled", sentinel: erc20types.ErrERC20TokenPairDisabled, solidityError: SolidityErrERC20TokenPairDisabled,
			translate: func(err error) error {
				return cosmosErrorRegistry.ResolveQueryError(p.ABI, TransferFromMethod, err, nil).Err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, returned := range []error{
				tc.sentinel,
				errorsmod.Wrap(tc.sentinel, "changed text"),
				fmt.Errorf("standard wrapper: %w", tc.sentinel),
			} {
				err := tc.translate(returned)
				carrier := err.(cmn.RevertDataCarrier)
				require.Equal(t, erc20ErrorSelector(tc.solidityError), carrier.RevertData())
				require.NotEqual(t, erc20ErrorSelector(cmn.SolidityErrMsgServerFailed), carrier.RevertData()[:4])
				require.NotEqual(t, erc20ErrorSelector(cmn.SolidityErrQueryFailed), carrier.RevertData()[:4])
				require.NotEqual(t, erc20ErrorSelector(cmn.SolidityErrUnmappedCosmosError), carrier.RevertData()[:4])
			}
		})
	}
}

func TestERC20UnmappedAndUnregisteredPathsRemainExplicit(t *testing.T) {
	p := Precompile{ABI: ABI}

	unmapped := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, TransferMethod, errSyntheticERC20Drift, nil).Err
	require.Equal(t, erc20ErrorSelector(cmn.SolidityErrUnmappedCosmosError), unmapped.(cmn.RevertDataCarrier).RevertData()[:4])

	internal := errors.New("infrastructure failure")
	msgErr := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, TransferMethod, internal, nil).Err
	require.Equal(t, erc20ErrorSelector(cmn.SolidityErrMsgServerFailed), msgErr.(cmn.RevertDataCarrier).RevertData()[:4])
	queryErr := cosmosErrorRegistry.ResolveQueryError(p.ABI, ApproveMethod, internal, nil).Err
	require.Equal(t, erc20ErrorSelector(cmn.SolidityErrQueryFailed), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
}

func erc20ErrorSelector(name string) []byte {
	definition := ABI.Errors[name]
	return definition.ID[:4]
}

func TestERC20BoundaryPreservationAndEquivalence(t *testing.T) {
	p := Precompile{ABI: ABI}
	t.Run("query", func(t *testing.T) {
		adapter := func(ctx sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveQueryError(p.ABI, "method", err, nil).Err
		}
		precompiletest.TestBoundaryAdapter(t, adapter)
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, "method", false, adapter, errSyntheticERC20Drift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	})
	t.Run("msg", func(t *testing.T) {
		adapter := func(ctx sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveMsgServerError(p.ABI, "method", err, nil).Err
		}
		precompiletest.TestBoundaryAdapter(t, adapter)
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, "method", true, adapter, errSyntheticERC20Drift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	})
}
