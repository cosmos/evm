package testutil

import (
	"fmt"
	"slices"

	"github.com/yihuang/go-abi"

	abci "github.com/cometbft/cometbft/abci/types"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// CheckLogs checks the logs for the given events and whether the transaction was successful or not.
func CheckLogs(logArgs LogCheckArgs) error {
	expABIEvents := logArgs.ExpEvents

	ethRes, err := evmtypes.DecodeTxResponse(logArgs.Res.Data)
	if err != nil {
		return fmt.Errorf("error while decoding ethereum tx response: %v", err)
	}

	reason, failed := CheckEthereumTxFailed(ethRes)
	if failed != !logArgs.ExpPass {
		return fmt.Errorf(
			"expected vm error found to be %t; got: %t (reason: %s)\nGas usage: %d/%d (~%d %%)",
			!logArgs.ExpPass,
			failed,
			reason,
			logArgs.Res.GasUsed,
			logArgs.Res.GasWanted,
			int64(float64(logArgs.Res.GasUsed)/float64(logArgs.Res.GasWanted)*100),
		)
	}

	if err := CheckVMError(logArgs.Res, "%s", logArgs.ErrContains); err != nil {
		return err
	}

	if len(ethRes.Logs) != len(logArgs.ExpEvents) {
		return fmt.Errorf("expected %d events in Ethereum response; got: %d", len(logArgs.ExpEvents), len(ethRes.Logs))
	}

	// Check if expected events are present in Ethereum response
	availableEventIDs := make([]string, 0, len(ethRes.Logs))
	for _, log := range ethRes.Logs {
		availableEventIDs = append(availableEventIDs, log.Topics[0])
	}

	expEventIDs := make([]string, 0, len(expABIEvents))
	for _, event := range expABIEvents {
		expEventIDs = append(expEventIDs, event.GetEventID().String())
	}

	for _, eventID := range expEventIDs {
		if !slices.Contains(availableEventIDs, eventID) {
			return fmt.Errorf("expected event with ID %v not found in Ethereum response", eventID)
		}
	}

	return nil
}

// LogCheckArgs is a struct that contains configuration for the log checking.
type LogCheckArgs struct {
	// ErrContains is the error message that is expected to be contained in the transaction response.
	ErrContains string
	// ExpEvents are the events which are expected to be emitted.
	ExpEvents []abi.Event
	// ExpPass is whether the transaction is expected to pass or not.
	ExpPass bool
	// Res is the response of the transaction.
	//
	// NOTE: This does not have to be set when using contracts.CallContractAndCheckLogs.
	Res abci.ExecTxResult
}

// WithErrContains sets the ErrContains field of LogCheckArgs.
// If any printArgs are provided, they are used to format the error message.
func (l LogCheckArgs) WithErrContains(errContains string, printArgs ...interface{}) LogCheckArgs {
	if len(printArgs) > 0 {
		errContains = fmt.Sprintf(errContains, printArgs...)
	}
	l.ErrContains = errContains
	return l
}

// WithErrNested append the ErrContains field of LogCheckArgs.
// If any printArgs are provided, they are used to format the error message.
func (l LogCheckArgs) WithErrNested(errContains string, printArgs ...interface{}) LogCheckArgs {
	if len(printArgs) > 0 {
		errContains = fmt.Sprintf(errContains, printArgs...)
	}
	l.ErrContains = fmt.Sprint(l.ErrContains, ": ", errContains)
	return l
}

// WithExpEvents sets the ExpEvents field of LogCheckArgs.
func (l LogCheckArgs) WithExpEvents(expEvents ...abi.Event) LogCheckArgs {
	l.ExpEvents = expEvents
	return l
}

// WithExpPass sets the ExpPass field of LogCheckArgs.
func (l LogCheckArgs) WithExpPass(expPass bool) LogCheckArgs {
	l.ExpPass = expPass
	return l
}

// WithRes sets the Res field of LogCheckArgs.
func (l LogCheckArgs) WithRes(res abci.ExecTxResult) LogCheckArgs {
	l.Res = res
	return l
}
