package v2

import (
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttime "github.com/cometbft/cometbft/types/time"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/testutil/constants"
	vmkeeper "github.com/cosmos/evm/x/vm/keeper"
	vmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/evm/x/vm/types/mocks"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
)

func TestMigrateStoreRemovesVestingPrecompile(t *testing.T) {
	ctx, keeper := setupKeeper(t)
	params := vmtypes.DefaultParams()
	params.ActiveStaticPrecompiles = append([]string{
		vmtypes.P256PrecompileAddress,
		vmtypes.Bech32PrecompileAddress,
		vmtypes.StakingPrecompileAddress,
		vmtypes.DistributionPrecompileAddress,
		vmtypes.ICS20PrecompileAddress,
		vestingPrecompileAddress,
	}, vmtypes.AvailableStaticPrecompiles[5:]...)
	require.NoError(t, keeper.SetParams(ctx, params))

	require.NoError(t, MigrateStore(ctx, keeper))

	require.Equal(t, vmtypes.AvailableStaticPrecompiles, keeper.GetParams(ctx).ActiveStaticPrecompiles)
}

func TestMigrateStoreNoopsWithoutVestingPrecompile(t *testing.T) {
	ctx, keeper := setupKeeper(t)
	params := vmtypes.DefaultParams()
	params.ActiveStaticPrecompiles = append([]string(nil), vmtypes.AvailableStaticPrecompiles...)
	require.NoError(t, keeper.SetParams(ctx, params))

	require.NoError(t, MigrateStore(ctx, keeper))

	require.Equal(t, params, keeper.GetParams(ctx))
}

func setupKeeper(t *testing.T) (sdk.Context, *vmkeeper.Keeper) {
	t.Helper()

	key := storetypes.NewKVStoreKey(vmtypes.StoreKey)
	oKey := storetypes.NewObjectStoreKey(vmtypes.ObjectKey)
	ctx := testutil.DefaultContextWithObjectStore(
		t,
		key,
		storetypes.NewTransientStoreKey("store_test"),
		oKey,
	).Ctx.WithBlockHeader(cmtproto.Header{Time: cmttime.Now()})

	accountKeeper := mocks.NewAccountKeeper(t)
	accountKeeper.On("GetModuleAddress", vmtypes.ModuleName).Return(sdk.AccAddress("evm"))

	encCfg := moduletestutil.MakeTestEncodingConfig()
	keeper := vmkeeper.NewKeeper(
		encCfg.Codec,
		key,
		oKey,
		[]storetypes.StoreKey{key, oKey},
		sdk.AccAddress("foobar"),
		accountKeeper,
		mocks.NewBankKeeper(t),
		mocks.NewStakingKeeper(t),
		mocks.NewFeeMarketKeeper(t),
		mocks.NewConsensusParamsKeeper(t),
		mocks.NewErc20Keeper(t),
		constants.EighteenDecimalsChainID,
		"",
	)

	return ctx, keeper
}
