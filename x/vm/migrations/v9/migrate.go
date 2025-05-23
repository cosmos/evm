package v9

import (
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	v8types "github.com/cosmos/evm/x/vm/migrations/v9/types"
	"github.com/cosmos/evm/x/vm/types"
)

// MigrateStore migrates the x/evm module state from the consensus version 5 to
// version 6. Specifically, it migrates the geth chain configuration
// that changed from geth v1.10 to v1.13.
func MigrateStore(
	ctx sdk.Context,
	storeKey storetypes.StoreKey,
	cdc codec.BinaryCodec,
) error {
	var v8Params v8types.V8Params
	store := ctx.KVStore(storeKey)
	bz := store.Get(types.KeyPrefixParams)

	cdc.MustUnmarshal(bz, &v8Params)

	accessCtrl := types.AccessControl{
		Create: types.AccessControlType{
			AccessType:        types.AccessType(v8Params.AccessControl.Create.AccessType),
			AccessControlList: v8Params.AccessControl.Create.AccessControlList,
		},
		Call: types.AccessControlType{
			AccessType:        types.AccessType(v8Params.AccessControl.Call.AccessType),
			AccessControlList: v8Params.AccessControl.Call.AccessControlList,
		},
	}

	updatedParams := types.Params{
		EvmDenom:                v8Params.EvmDenom,
		ExtraEIPs:               v8Params.ExtraEIPs,
		AllowUnprotectedTxs:     v8Params.AllowUnprotectedTxs,
		EVMChannels:             v8Params.EVMChannels,
		AccessControl:           accessCtrl,
		ActiveStaticPrecompiles: v8Params.ActiveStaticPrecompiles,
	}

	if err := updatedParams.Validate(); err != nil {
		return err
	}
	updatedBz := cdc.MustMarshal(&updatedParams)
	store.Set(types.KeyPrefixParams, updatedBz)

	return nil
}
