package erc20

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	erc20types "github.com/cosmos/evm/x/erc20/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

var errSyntheticERC20Drift = errorsmod.Register("erc20-phase-three-drift", 77, "unstable reason")

func TestTranslateERC20RegisteredErrorsDirectAndWrapped(t *testing.T) {
	p := Precompile{ABI: ABI}
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())
	tests := []struct {
		name          string
		sentinel      error
		solidityError string
		translate     func(error) error
	}{
		{
			name: "bank send disabled", sentinel: banktypes.ErrSendDisabled, solidityError: SolidityErrBankSendDisabled,
			translate: func(err error) error { return p.erc20MsgError(ctx, TransferMethod, err) },
		},
		{
			name: "token pair not found", sentinel: erc20types.ErrTokenPairNotFound, solidityError: SolidityErrERC20TokenPairNotFound,
			translate: func(err error) error { return p.erc20QueryError(ctx, ApproveMethod, err) },
		},
		{
			name: "token pair disabled", sentinel: erc20types.ErrERC20TokenPairDisabled, solidityError: SolidityErrERC20TokenPairDisabled,
			translate: func(err error) error { return p.erc20QueryError(ctx, TransferFromMethod, err) },
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
	var output bytes.Buffer
	ctx := sdk.Context{}.WithLogger(log.NewLogger(&output, log.OutputJSONOption()))
	p := Precompile{ABI: ABI}

	unmapped := p.erc20MsgError(ctx, TransferMethod, errSyntheticERC20Drift)
	require.Equal(t, erc20ErrorSelector(cmn.SolidityErrUnmappedCosmosError), unmapped.(cmn.RevertDataCarrier).RevertData()[:4])
	logs := output.String()
	require.Equal(t, 1, strings.Count(logs, "unmapped registered Cosmos error"))
	require.Contains(t, logs, `"precompile":"erc20"`)
	require.Contains(t, logs, `"method":"transfer"`)
	require.Contains(t, logs, `"codespace":"erc20-phase-three-drift"`)
	require.Contains(t, logs, `"code":77`)
	require.NotContains(t, logs, "unstable reason")
	_ = p.erc20MsgError(ctx, TransferMethod, banktypes.ErrSendDisabled)
	require.Equal(t, 1, strings.Count(output.String(), "unmapped registered Cosmos error"), "known mappings must not emit the unmapped signal")

	internal := errors.New("infrastructure failure")
	msgErr := p.erc20MsgError(ctx, TransferMethod, internal)
	require.Equal(t, erc20ErrorSelector(cmn.SolidityErrMsgServerFailed), msgErr.(cmn.RevertDataCarrier).RevertData()[:4])
	queryErr := p.erc20QueryError(ctx, ApproveMethod, internal)
	require.Equal(t, erc20ErrorSelector(cmn.SolidityErrQueryFailed), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
}

func erc20ErrorSelector(name string) []byte {
	definition := ABI.Errors[name]
	return definition.ID[:4]
}
