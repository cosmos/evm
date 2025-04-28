package v9_test

import (
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
	encCfg := encoding.MakeConfig()
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
		EvmDenom: defaultParams.EvmDenom,
		ChainConfig: v8types.V8ChainConfig{
			ChainId:             defaultParams.ChainConfig.ChainId,
			Denom:               defaultParams.ChainConfig.Denom,
			Decimals:            defaultParams.ChainConfig.Decimals,
			HomesteadBlock:      defaultParams.ChainConfig.HomesteadBlock,
			DAOForkBlock:        defaultParams.ChainConfig.DAOForkBlock,
			DAOForkSupport:      defaultParams.ChainConfig.DAOForkSupport,
			EIP150Block:         defaultParams.ChainConfig.EIP150Block,
			EIP155Block:         defaultParams.ChainConfig.EIP155Block,
			EIP158Block:         defaultParams.ChainConfig.EIP158Block,
			ByzantiumBlock:      defaultParams.ChainConfig.ByzantiumBlock,
			ConstantinopleBlock: defaultParams.ChainConfig.ConstantinopleBlock,
			PetersburgBlock:     defaultParams.ChainConfig.PetersburgBlock,
			IstanbulBlock:       defaultParams.ChainConfig.IstanbulBlock,
			MuirGlacierBlock:    defaultParams.ChainConfig.MuirGlacierBlock,
			BerlinBlock:         defaultParams.ChainConfig.BerlinBlock,
			LondonBlock:         defaultParams.ChainConfig.LondonBlock,
			ArrowGlacierBlock:   defaultParams.ChainConfig.ArrowGlacierBlock,
			GrayGlacierBlock:    defaultParams.ChainConfig.GrayGlacierBlock,
			MergeNetsplitBlock:  defaultParams.ChainConfig.MergeNetsplitBlock,
			ShanghaiBlock:       defaultParams.ChainConfig.ShanghaiTime,
			CancunBlock:         defaultParams.ChainConfig.CancunTime,
		},
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
