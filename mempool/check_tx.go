package mempool

import (
	"context"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// NewCheckTxHandler creates a CheckTx handler that integrates with the EVM mempool for transaction validation.
// It routes new CheckTx requests through the same async insert worker path used by
// the app-side mempool and waits for the insert result.
func NewCheckTxHandler(mempool *ExperimentalEVMMempool, debug bool, timeout time.Duration) types.CheckTxHandler {
	if timeout <= 0 {
		panic("invalid timeout CheckTxHandler timeout value")
	}
	return func(_ types.RunTx, request *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
		tx, err := mempool.txConfig.TxDecoder()(request.Tx)
		if err != nil {
			return sdkerrors.ResponseCheckTxWithEvents(err, 0, 0, nil, debug), nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := mempool.Insert(ctx, tx); err != nil {
			return sdkerrors.ResponseCheckTxWithEvents(err, 0, 0, nil, debug), nil
		}
		return &abci.ResponseCheckTx{Code: abci.CodeTypeOK}, nil
	}
}
