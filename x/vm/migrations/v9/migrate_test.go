package v9_test

import (
	"github.com/cosmos/evm/server/config"
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/encoding"
	v9 "github.com/cosmos/evm/x/vm/migrations/v9"
	v8types "github.com/cosmos/evm/x/vm/migrations/v9/types"
	"github.com/cosmos/evm/x/vm/types"
)

func TestMigrate(t *testing.T) {
	encCfg := encoding.MakeConfig(config.DefaultEVMChainID)
	cdc := encCfg.Codec

	storeKey := storetypes.NewKVStoreKey(types.ModuleName)
	tKey := storetypes.NewTransientStoreKey("transient_test")
	ctx := testutil.DefaultContext(storeKey, tKey)
	kvStore := ctx.KVStore(storeKey)

	// set v8 defaultParams
	defaultParams := types.DefaultParams()
	accessCtrl := v8types.V8AccessControl{
		Create: v8types.V8AccessControlType{
			AccessType:        v8types.V8AccessType(defaultParams.AccessControl.Create.AccessType),
			AccessControlList: defaultParams.AccessControl.Create.AccessControlList,
		},
		Call: v8types.V8AccessControlType{
			AccessType:        v8types.V8AccessType(defaultParams.AccessControl.Call.AccessType),
			AccessControlList: defaultParams.AccessControl.Call.AccessControlList,
		},
	}

	v8Params := v8types.V8Params{
		EvmDenom:            defaultParams.EvmDenom,
		AllowUnprotectedTxs: defaultParams.AllowUnprotectedTxs,
		AccessControl:       accessCtrl,
	}

	// Set the params in the store
	bz := cdc.MustMarshal(&v8Params)
	kvStore.Set(types.KeyPrefixParams, bz)

	// Migrate the store
	err := v9.MigrateStore(ctx, storeKey, cdc)
	require.NoError(t, err)

	var updatedParams types.Params
	paramsBz := kvStore.Get(types.KeyPrefixParams)
	cdc.MustUnmarshal(paramsBz, &updatedParams)

	// test that the params have been migrated correctly
	require.Equal(t, defaultParams, updatedParams)
}
