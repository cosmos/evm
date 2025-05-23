package keeper

import (
	"context"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Enforce that Keeper implements the expected keeper interfaces
var _ evmtypes.BankKeeper = Keeper{}

// Keeper defines the precisebank module's keeper
type Keeper struct {
	cdc      codec.BinaryCodec
	storeKey storetypes.StoreKey

	bk types.BankKeeper
	ak types.AccountKeeper
}

func (k Keeper) GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins {
	return k.bk.GetAllBalances(ctx, addr)
}

func (k Keeper) SpendableBalances(ctx context.Context, req *banktypes.QuerySpendableBalancesRequest) (*banktypes.QuerySpendableBalancesResponse, error) {
	return k.bk.SpendableBalances(ctx, req)
}

func (k Keeper) BlockedAddr(addr sdk.AccAddress) bool {
	return k.bk.BlockedAddr(addr)
}

// NewKeeper creates a new keeper
func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey storetypes.StoreKey,
	bk types.BankKeeper,
	ak types.AccountKeeper,
) Keeper {
	return Keeper{
		cdc:      cdc,
		storeKey: storeKey,
		bk:       bk,
		ak:       ak,
	}
}

func (k Keeper) IterateTotalSupply(ctx context.Context, cb func(coin sdk.Coin) bool) {
	k.bk.IterateTotalSupply(ctx, cb)
}

func (k Keeper) GetSupply(ctx context.Context, denom string) sdk.Coin {
	return k.bk.GetSupply(ctx, denom)
}
