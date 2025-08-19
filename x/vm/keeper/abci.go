package keeper

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethparams "github.com/ethereum/go-ethereum/params"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BeginBlock emits a base fee event which will be adjusted to the evm decimals
func (k *Keeper) BeginBlock(ctx sdk.Context) error {
	logger := ctx.Logger().With("begin_block", "evm")

	// Base fee is already set on FeeMarket BeginBlock
	// that runs before this one
	// We emit this event on the EVM and FeeMarket modules
	// because they can be different if the evm denom has 6 decimals
	res, err := k.BaseFee(ctx, &evmtypes.QueryBaseFeeRequest{})
	if err != nil {
		logger.Error("error when getting base fee", "error", err.Error())
	}
	if res != nil && res.BaseFee != nil && !res.BaseFee.IsNil() {
		// Store current base fee in event
		ctx.EventManager().EmitEvents(sdk.Events{
			sdk.NewEvent(
				evmtypes.EventTypeFeeMarket,
				sdk.NewAttribute(evmtypes.AttributeKeyBaseFee, res.BaseFee.String()),
			),
		})
	}

	acct := k.GetAccount(ctx, ethparams.HistoryStorageAddress)
	if acct != nil && acct.IsContract() {
		// set current block hash in the contract storage, compatible with EIP-2935
		ringIndex := uint64(ctx.BlockHeight() % ethparams.HistoryServeWindow) //nolint:gosec // G115 // won't exceed uint64
		var key common.Hash
		binary.BigEndian.PutUint64(key[24:], ringIndex)
		k.SetState(ctx, ethparams.HistoryStorageAddress, key, ctx.HeaderHash())
	}
	return nil
}

// EndBlock also retrieves the bloom filter value from the transient store and commits it to the
// KVStore. The EVM end block logic doesn't update the validator set, thus it returns
// an empty slice.
func (k *Keeper) EndBlock(ctx sdk.Context) error {
	// Gas costs are handled within msg handler so costs should be ignored
	infCtx := ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())

	if k.evmMempool != nil {
		k.evmMempool.GetBlockchain().NotifyNewBlock()
	}

	bloom := ethtypes.BytesToBloom(k.GetBlockBloomTransient(infCtx).Bytes())
	k.EmitBlockBloomEvent(infCtx, bloom)

	return nil
}
