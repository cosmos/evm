package keeper

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/store/prefix"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// IsContract determines if the given address is a smart contract.
func (k *Keeper) IsContract(ctx sdk.Context, addr common.Address) bool {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixCodeHash)
	return store.Has(addr.Bytes())
}
