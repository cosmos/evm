package keeper

import (
	"github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetEvmCoinInfo returns the EVM Coin Info stored in the module
func (k Keeper) GetEvmCoinInfo(ctx sdk.Context) (coinInfo types.EvmCoinInfo) {
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.KeyPrefixEvmCoinInfo)
	if bz == nil {
		return coinInfo
	}
	k.cdc.MustUnmarshal(bz, &coinInfo)
	return
}

// SetEvmCoinInfo sets the EVM Coin Info stored in the module
func (k Keeper) SetEvmCoinInfo(ctx sdk.Context, coinInfo types.EvmCoinInfo) error {
	store := ctx.KVStore(k.storeKey)
	bz, err := k.cdc.Marshal(&coinInfo)
	if err != nil {
		return err
	}

	store.Set(types.KeyPrefixEvmCoinInfo, bz)
	return nil
}
