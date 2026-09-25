package evm

import (
	"strconv"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EmitTxHashEvent emits the ethereum_tx event with the tx hash and the
// cosmos-tx index from ctx.TxIndex(). Emitting from the ante handler ensures
// expected-failure txs still produce the event for indexer lookups.
func EmitTxHashEvent(ctx sdk.Context, msg *evmtypes.MsgEthereumTx, blockTxIndex uint64) {
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			evmtypes.EventTypeEthereumTx,
			sdk.NewAttribute(evmtypes.AttributeKeyEthereumTxHash, msg.AsTransaction().Hash().String()),
			sdk.NewAttribute(evmtypes.AttributeKeyTxIndex, strconv.FormatUint(blockTxIndex, 10)), // #nosec G115
		),
	)
}
