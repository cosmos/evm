package keeper

import (
	"context"

	"github.com/cosmos/evm/x/epixmint/types"

	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Keeper of the epixmint store
type Keeper struct {
	cdc      codec.BinaryCodec
	storeKey storetypes.StoreKey

	bankKeeper         types.BankKeeper
	accountKeeper      types.AccountKeeper
	distributionKeeper types.DistributionKeeper
	stakingKeeper      types.StakingKeeper

	// the address capable of executing a MsgUpdateParams message. Typically, this
	// should be the x/gov module account.
	authority string
}

// NewKeeper creates a new epixmint Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	key storetypes.StoreKey,
	bankKeeper types.BankKeeper,
	accountKeeper types.AccountKeeper,
	distributionKeeper types.DistributionKeeper,
	stakingKeeper types.StakingKeeper,
	authority string,
) Keeper {
	return Keeper{
		cdc:                cdc,
		storeKey:           key,
		bankKeeper:         bankKeeper,
		accountKeeper:      accountKeeper,
		distributionKeeper: distributionKeeper,
		stakingKeeper:      stakingKeeper,
		authority:          authority,
	}
}

// GetAuthority returns the x/epixmint module's authority.
func (k Keeper) GetAuthority() string {
	return k.authority
}

// GetModuleAddress returns the address of the specified module account.
func (k Keeper) GetModuleAddress(moduleName string) sdk.AccAddress {
	return k.accountKeeper.GetModuleAddress(moduleName)
}

// GetParams returns the total set of epixmint parameters.
func (k Keeper) GetParams(ctx context.Context) (params types.Params) {
	store := k.storeService(ctx)
	bz := store.Get(types.ParamsKey)
	if bz == nil {
		return params
	}

	k.cdc.MustUnmarshal(bz, &params)
	return params
}

// SetParams sets the total set of epixmint parameters.
func (k Keeper) SetParams(ctx context.Context, params types.Params) error {
	if err := params.Validate(); err != nil {
		return err
	}

	store := k.storeService(ctx)
	bz := k.cdc.MustMarshal(&params)
	store.Set(types.ParamsKey, bz)

	return nil
}

// storeService returns the store service for the epixmint module.
func (k Keeper) storeService(ctx context.Context) storetypes.KVStore {
	return sdk.UnwrapSDKContext(ctx).KVStore(k.storeKey)
}
