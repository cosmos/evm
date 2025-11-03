package evm

import (
	"strconv"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EmitTxHashEvent emits the Ethereum tx
//
// FIXME: This is Technical debt. Ideally the sdk.Tx hash should be the Ethereum
// tx hash (msg.Hash) instead of using events for indexing Eth txs.
func EmitTxHashEvent(ctx sdk.Context, msg *evmtypes.MsgEthereumTx, blockTxIndex uint64) {
	// emit ethereum tx hash as an event so that it can be indexed by CometBFT for query purposes
	// it's emitted in ante handler, so we can query failed transaction (out of block gas limit).
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			evmtypes.EventTypeEthereumTx,
			sdk.NewAttribute(evmtypes.AttributeKeyEthereumTxHash, msg.Hash().String()),
			sdk.NewAttribute(evmtypes.AttributeKeyTxIndex, strconv.FormatUint(blockTxIndex, 10)), // #nosec G115
		),
	)
}
