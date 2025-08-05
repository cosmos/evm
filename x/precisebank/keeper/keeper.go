package keeper

import (
	"context"

	"github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
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

// BANK KEEPER INTERFACE PASSTHROUGHS

func (k Keeper) IterateTotalSupply(ctx context.Context, cb func(coin sdk.Coin) bool) {
	k.bk.IterateTotalSupply(ctx, cb)
}

func (k Keeper) GetSupply(ctx context.Context, denom string) sdk.Coin {
	return k.bk.GetSupply(ctx, denom)
}

func (k Keeper) GetDenomMetaData(ctx context.Context, denom string) (banktypes.Metadata, bool) {
	return k.bk.GetDenomMetaData(ctx, denom)
}

func (k Keeper) SetDenomMetaData(ctx context.Context, denomMetaData banktypes.Metadata) {
	k.bk.SetDenomMetaData(ctx, denomMetaData)
}