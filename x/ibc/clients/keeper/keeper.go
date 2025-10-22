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

	// state management
	Schema collections.Schema
	Params collections.Item[types.Params]
	// Mapping from client ID to ClientPrecompile
	ClientPrecompiles collections.Map[string, types.ClientPrecompile]
	// Mapping from precompile address to ClientPrecompile
	Precompiles collections.Map[[]byte, types.ClientPrecompile]
}

// NewKeeper creates a new Keeper instance
func NewKeeper(cdc codec.BinaryCodec, addressCodec address.Codec, storeService storetypes.KVStoreService, authority string) Keeper {
	if _, err := addressCodec.StringToBytes(authority); err != nil {
		panic(fmt.Errorf("invalid authority address: %w", err))
	}

	sb := collections.NewSchemaBuilder(storeService)
	k := Keeper{
		cdc:               cdc,
		addressCodec:      addressCodec,
		authority:         authority,
		Params:            collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		ClientPrecompiles: collections.NewMap(sb, types.ClientPrecompilesKey, "client_precompiles", collections.StringKey, codec.CollValue[types.ClientPrecompile](cdc)),
		Precompiles:       collections.NewMap(sb, types.PrecompilesKey, "precompiles", collections.BytesKey, codec.CollValue[types.ClientPrecompile](cdc)),
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
