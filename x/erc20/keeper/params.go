package keeper

import (
	"github.com/cosmos/evm/x/erc20/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var isTrue = []byte("0x01")

// GetParams returns the total set of erc20 parameters.
func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	enableErc20 := k.IsERC20Enabled(ctx)
	permissionlessRegistration := k.isPermissionlessRegistration(ctx)
	return types.NewParams(enableErc20, permissionlessRegistration)
}

// SetParams sets the erc20 parameters to the param space.
func (k Keeper) SetParams(ctx sdk.Context, newParams types.Params) error {
	k.setERC20Enabled(ctx, newParams.EnableErc20)
	k.SetPermissionlessRegistration(ctx, newParams.PermissionlessRegistration)
	return nil
}

// IsERC20Enabled returns true if the module logic is enabled
func (k Keeper) IsERC20Enabled(ctx sdk.Context) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(types.ParamStoreKeyEnableErc20)
}

// setERC20Enabled sets the EnableERC20 param in the store
func (k Keeper) setERC20Enabled(ctx sdk.Context, enable bool) {
	store := ctx.KVStore(k.storeKey)
	if enable {
		store.Set(types.ParamStoreKeyEnableErc20, isTrue)
		return
	}
	store.Delete(types.ParamStoreKeyEnableErc20)
}

// isPermissionlessRegistration returns true if the module enabled permissionless
// erc20 registration
func (k Keeper) isPermissionlessRegistration(ctx sdk.Context) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(types.ParamStoreKeyPermissionlessRegistration)
}

func (k Keeper) SetPermissionlessRegistration(ctx sdk.Context, permissionlessRegistration bool) {
	store := ctx.KVStore(k.storeKey)
	if permissionlessRegistration {
		store.Set(types.ParamStoreKeyPermissionlessRegistration, isTrue)
		return
	}
	store.Delete(types.ParamStoreKeyPermissionlessRegistration)
}
