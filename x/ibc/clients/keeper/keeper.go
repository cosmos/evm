package keeper

import (
	"fmt"

	"github.com/cosmos/evm/x/ibc/clients/types"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	storetypes "cosmossdk.io/core/store"
	"github.com/cosmos/cosmos-sdk/codec"
)

type Keeper struct {
	cdc          codec.BinaryCodec
	addressCodec address.Codec

	// authority is the address capable of executing a MsgUpdateParams and other authority-gated message.
	authority string

	clientKeeper types.ClientKeeper

	// state management
	Schema     collections.Schema
	ParamsItem collections.Item[types.Params]
	// Mapping from client ID to ClientPrecompile
	ClientPrecompilesMap collections.Map[string, types.ClientPrecompile]
	// Mapping from precompile address to ClientPrecompile
	AddressPrecompilesMap collections.Map[[]byte, types.ClientPrecompile]
}

// NewKeeper creates a new Keeper instance
func NewKeeper(cdc codec.BinaryCodec, addressCodec address.Codec, storeService storetypes.KVStoreService, authority string, clientKeeper types.ClientKeeper) Keeper {
	if _, err := addressCodec.StringToBytes(authority); err != nil {
		panic(fmt.Errorf("invalid authority address: %w", err))
	}

	sb := collections.NewSchemaBuilder(storeService)
	k := Keeper{
		cdc:                   cdc,
		addressCodec:          addressCodec,
		authority:             authority,
		clientKeeper:          clientKeeper,
		ParamsItem:            collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		ClientPrecompilesMap:  collections.NewMap(sb, types.ClientPrecompilesKey, "client_precompiles", collections.StringKey, codec.CollValue[types.ClientPrecompile](cdc)),
		AddressPrecompilesMap: collections.NewMap(sb, types.PrecompilesKey, "address_precompiles", collections.BytesKey, codec.CollValue[types.ClientPrecompile](cdc)),
	}

	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}

	k.Schema = schema

	return k
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() string {
	return k.authority
}
