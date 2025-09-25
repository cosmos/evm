package keeper

import (
	"github.com/cosmos/evm/x/ibc/transfer/types"
	"github.com/cosmos/ibc-go/v10/modules/apps/transfer/keeper"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"

	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"

	"github.com/cosmos/cosmos-sdk/codec"
)

// Keeper defines the modified IBC transfer keeper that embeds the original one.
// It also contains the bank keeper and the erc20 keeper to support ERC20 tokens
// to be sent via IBC.
type Keeper struct {
	*keeper.Keeper
	bankKeeper    types.BankKeeper
	erc20Keeper   types.ERC20Keeper
	accountKeeper types.AccountKeeper
}

// NewKeeper creates a new IBC transfer Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	addressCodec address.Codec,
	storeService corestore.KVStoreService,

	channelKeeper transfertypes.ChannelKeeper,
	msgRouter transfertypes.MessageRouter,
	authKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
	erc20Keeper types.ERC20Keeper,
	authority string,
) Keeper {
	// create the original IBC transfer keeper for embedding
	transferKeeper := keeper.NewKeeper(
		cdc, addressCodec, storeService,
		channelKeeper, msgRouter,
		authKeeper, bankKeeper, authority,
	)

	return Keeper{
		Keeper:        transferKeeper,
		bankKeeper:    bankKeeper,
		erc20Keeper:   erc20Keeper,
		accountKeeper: authKeeper,
	}
}
