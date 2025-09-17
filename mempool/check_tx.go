package mempool

import (
	"errors"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// NewCheckTxHandler creates a CheckTx handler that integrates with the EVM mempool for transaction validation.
// It wraps the standard transaction execution flow to handle EVM-specific nonce gap errors by routing
// transactions with higher tx sequence numbers to the mempool for potential future execution.
// Returns a handler function that processes ABCI CheckTx requests and manages EVM transaction sequencing.
func NewCheckTxHandler(mempool *ExperimentalEVMMempool) types.CheckTxHandler {
	return func(runTx types.RunTx, request *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
		gInfo, result, anteEvents, err := runTx(request.Tx, nil)
		if err != nil {
			// detect if there is a nonce gap error (only returned for EVM transactions)
			if errors.Is(err, ErrNonceGap) || errors.Is(err, ErrNonceLow) {
				// send it to the mempool for further triage
				err := mempool.InsertInvalidNonce(request.Tx)
				if err != nil {
					return sdkerrors.ResponseCheckTxWithEvents(err, gInfo.GasWanted, gInfo.GasUsed, anteEvents, false), nil
				}
			}
			// anything else, return regular error
			return sdkerrors.ResponseCheckTxWithEvents(err, gInfo.GasWanted, gInfo.GasUsed, anteEvents, false), nil
		}

		return &abci.ResponseCheckTx{
			GasWanted: int64(gInfo.GasWanted), // #nosec G115 -- this is copied from the Cosmos SDK
			GasUsed:   int64(gInfo.GasUsed),   // #nosec G115 -- this is copied from the Cosmos SDK
			Log:       result.Log,
			Data:      result.Data,
			Events:    types.MarkEventsToIndex(result.Events, nil),
		}, nil
	}
}
