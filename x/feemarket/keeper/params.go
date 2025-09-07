package keeper

import (
	"github.com/cosmos/evm/x/feemarket/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetParams returns the total set of fee market parameters.
func (k Keeper) GetParams(ctx sdk.Context) types.Params {
	var params *types.Params
	objStore := ctx.ObjectStore(k.objectKey)
	v := objStore.Get(types.KeyPrefixObjectParams)
	if v == nil {
		params = new(types.Params)
		bz := ctx.KVStore(k.storeKey).Get(types.ParamsKey)
		if bz != nil {
			k.cdc.MustUnmarshal(bz, params)
		}
		objStore.Set(types.KeyPrefixObjectParams, params)
	} else {
		params = v.(*types.Params)
	}
	return *params
}

// SetParams sets the fee market params in a single key
func (k Keeper) SetParams(ctx sdk.Context, p types.Params) error {
	if err := p.Validate(); err != nil {
		return err
	}
	store := ctx.KVStore(k.storeKey)
	bz := k.cdc.MustMarshal(&p)
	store.Set(types.ParamsKey, bz)

	// set to cache as well, decode again to be compatible with the previous behavior
	var params types.Params
	k.cdc.MustUnmarshal(bz, &params)
	ctx.ObjectStore(k.objectKey).Set(types.KeyPrefixObjectParams, &params)

	return nil
}

// ----------------------------------------------------------------------------
// Parent Base Fee
// Required by EIP1559 base fee calculation.
// ----------------------------------------------------------------------------

// GetBaseFeeEnabled returns true if base fee is enabled
func (k Keeper) GetBaseFeeEnabled(ctx sdk.Context) bool {
	params := k.GetParams(ctx)
	return !params.NoBaseFee && ctx.BlockHeight() >= params.EnableHeight
}

// GetBaseFee gets the base fee from the store
func (k Keeper) GetBaseFee(ctx sdk.Context) math.LegacyDec {
	params := k.GetParams(ctx)
	if params.NoBaseFee {
		return math.LegacyDec{}
	}
	return params.BaseFee
}

// SetBaseFee set's the base fee in the store
func (k Keeper) SetBaseFee(ctx sdk.Context, baseFee math.LegacyDec) {
	params := k.GetParams(ctx)
	params.BaseFee = baseFee
	err := k.SetParams(ctx, params)
	if err != nil {
		return
	}
}
