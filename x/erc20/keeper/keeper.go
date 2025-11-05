package keeper

import (
	"fmt"

	"github.com/cosmos/evm/x/erc20/types"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
)

// Keeper of this module maintains collections of erc20.
type Keeper struct {
	storeKey storetypes.StoreKey
	cdc      codec.BinaryCodec
	// the address capable of executing a MsgUpdateParams message. Typically, this should be the x/gov module account.
	authority sdk.AccAddress

	accountKeeper  types.AccountKeeper
	bankKeeper     bankkeeper.Keeper
	evmKeeper      types.EVMKeeper
	stakingKeeper  types.StakingKeeper
	transferKeeper *transferkeeper.Keeper
}

// NewKeeper creates new instances of the erc20 Keeper
func NewKeeper(
	storeKey storetypes.StoreKey,
	cdc codec.BinaryCodec,
	authority sdk.AccAddress,
	ak types.AccountKeeper,
	bk bankkeeper.Keeper,
	evmKeeper types.EVMKeeper,
	sk types.StakingKeeper,
	transferKeeper *transferkeeper.Keeper,
) Keeper {
	// ensure gov module account is set and is not nil
	if err := sdk.VerifyAddressFormat(authority); err != nil {
		panic(err)
	}

	return Keeper{
		authority:      authority,
		storeKey:       storeKey,
		cdc:            cdc,
		accountKeeper:  ak,
		bankKeeper:     bk,
		evmKeeper:      evmKeeper,
		stakingKeeper:  sk,
		transferKeeper: transferKeeper,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}
