package factory

import (
	"bytes"
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/testutil"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"
)

// buildMsgEthereumTx builds an Ethereum transaction from the given arguments and populates the From field.
func buildMsgEthereumTx(txArgs evmtypes.EvmTxArgs, fromAddr common.Address) evmtypes.MsgEthereumTx {
	msgEthereumTx := evmtypes.NewTx(&txArgs)
	msgEthereumTx.From = fromAddr.Bytes()
	return *msgEthereumTx
}

// CheckError is a helper function to check if the error is the expected one.
func CheckError(err error, logCheckArgs testutil.LogCheckArgs) error {
	switch {
	case logCheckArgs.ExpPass && err == nil:
		return nil
	case !logCheckArgs.ExpPass && err == nil:
		return errorsmod.Wrap(err, "expected error but got none")
	case logCheckArgs.ExpPass && err != nil:
		return errorsmod.Wrap(err, "expected no error but got one")
	case logCheckArgs.ErrExact != nil:
		// When ErrExact is provided, validate revert bytes exactly (e.g. Solidity custom errors).
		// This intentionally does not fall back to substring matching.
		if err == nil {
			return errorsmod.Wrap(err, "expected error but got none")
		}

		var gotCarrier cmn.RevertDataCarrier
		var wantCarrier cmn.RevertDataCarrier
		if !errors.As(logCheckArgs.ErrExact, &wantCarrier) {
			return errorsmod.Wrapf(err, "expected want error to implement RevertDataCarrier; want=%T", logCheckArgs.ErrExact)
		}

		// The broadcast/tx execution path wraps errors (e.g. "failed ETH tx: ...") and often
		// does not preserve a RevertDataCarrier on the returned error value. However, we still
		// have the ABCI tx result available and can decode the raw EVM revert bytes from it.
		if !errors.As(err, &gotCarrier) {
			ethRes, decErr := evmtypes.DecodeTxResponse(logCheckArgs.Res.Data)
			if decErr != nil {
				return errorsmod.Wrapf(err, "expected errors to implement RevertDataCarrier; got=%T want=%T", err, logCheckArgs.ErrExact)
			}
			gotCarrier = evmtypes.NewExecErrorWithReason(ethRes.Ret)
		}
		if !bytes.Equal(gotCarrier.RevertData(), wantCarrier.RevertData()) {
			return errorsmod.Wrapf(err, "expected revert data mismatch (got=%x want=%x)", gotCarrier.RevertData(), wantCarrier.RevertData())
		}
		return nil
	case logCheckArgs.ErrContains == "":
		// NOTE: if err contains is empty, we return the error as it is
		return errorsmod.Wrap(err, "ErrContains needs to be filled")
	case err == nil:
		panic("unexpected state: err is nil; this should not happen")
	case !strings.Contains(err.Error(), logCheckArgs.ErrContains):
		return errorsmod.Wrapf(err, "expected different error; wanted %q", logCheckArgs.ErrContains)
	}

	return nil
}
