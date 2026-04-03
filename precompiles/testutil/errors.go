package testutil

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// CheckVMError is a helper function used to check if the transaction is reverted with the expected error message
// in the VmError field of the MsgEthereumResponse struct.
func CheckVMError(res abci.ExecTxResult, expErrMsg string, args ...interface{}) error {
	if !res.IsOK() {
		return fmt.Errorf("code 0 was expected on response but got code %d", res.Code)
	}
	ethRes, err := evmtypes.DecodeTxResponse(res.Data)
	if err != nil {
		return fmt.Errorf("error occurred while decoding the TxResponse. %s", err)
	}
	expMsg := fmt.Sprintf(expErrMsg, args...)
	if !strings.Contains(ethRes.VmError, expMsg) {
		return fmt.Errorf("unexpected VmError on response. expected error to contain: %s, received: %s", expMsg, ethRes.VmError)
	}
	return nil
}

// CheckEthereumTxFailed checks if there is a VM error in the transaction response and returns the reason.
func CheckEthereumTxFailed(ethRes *evmtypes.MsgEthereumTxResponse) (string, bool) {
	reason := ethRes.VmError
	return reason, reason != ""
}

// RequireExactError asserts exact error equality for expected precompile reverts.
// If both errors carry revert data, it compares ABI-encoded revert bytes exactly.
func RequireExactError(t *testing.T, got error, want error) {
	t.Helper()
	require.Error(t, got)
	require.NotNil(t, want)

	var gotCarrier cmn.RevertDataCarrier
	var wantCarrier cmn.RevertDataCarrier
	if errors.As(got, &gotCarrier) && errors.As(want, &wantCarrier) {
		require.Equal(t, wantCarrier.RevertData(), gotCarrier.RevertData())
		return
	}

	require.EqualError(t, got, want.Error())
}
