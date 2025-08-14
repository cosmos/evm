package ante

import (
	"github.com/ethereum/go-ethereum/common"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type PendingTxListener func(common.Hash)

type TxListenerDecorator struct {
	pendingTxListener PendingTxListener
}

// NewTxListenerDecorator creates a new TxListenerDecorator with the provided PendingTxListener.
// CONTRACT: must be put at the last of the chained decorators
func NewTxListenerDecorator(pendingTxListener PendingTxListener) TxListenerDecorator {
	return TxListenerDecorator{pendingTxListener}
}

func (d TxListenerDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	if ctx.IsReCheckTx() {
		return next(ctx, tx, simulate)
	}
	if ctx.IsCheckTx() && !simulate && d.pendingTxListener != nil {
		for _, msg := range tx.GetMsgs() {
			if ethTx, ok := msg.(*evmtypes.MsgEthereumTx); ok {
				d.pendingTxListener(ethTx.Hash())
			}
		}
	}
	return next(ctx, tx, simulate)
}
