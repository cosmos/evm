package common_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	cmnmocks "github.com/cosmos/evm/precompiles/common/mocks"
	testutil "github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/evm/x/vm/types/mocks"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// TestBalanceHandlerNestedPrecompile checks that when a precompile action
// makes a nested EVM call that runs another precompile, the outer
// BalanceHandler applies its own bank events and does not re-apply the
// events already applied by the inner BalanceHandler.
func TestBalanceHandlerNestedPrecompile(t *testing.T) {
	setupBalanceHandlerTest(t)

	ctx := sdktestutil.DefaultContext(storetypes.NewKVStoreKey("test"), storetypes.NewTransientStoreKey("test_t"))
	_, addrs, err := testutil.GeneratePrivKeyAddressPairs(2)
	require.NoError(t, err)
	spenderAcc, receiverAcc := addrs[0], addrs[1]
	spender := common.BytesToAddress(spenderAcc)
	receiver := common.BytesToAddress(receiverAcc)

	db := statedb.New(ctx, mocks.NewEVMKeeper(), statedb.NewEmptyTxConfig())
	db.AddBalance(spender, uint256.NewInt(100), tracing.BalanceChangeUnspecified)
	require.Equal(t, uint64(0), db.GetBalance(receiver).Uint64())

	bankKeeper := cmnmocks.NewBankKeeper(t)
	bankKeeper.Mock.On("BlockedAddr", mock.AnythingOfType("types.AccAddress")).Return(false).Maybe()
	factory := cmn.NewBalanceHandlerFactory(bankKeeper)

	enterPrecompile := func() (sdk.Context, *cmn.BalanceHandler) {
		cacheCtx, err := db.GetCacheContext()
		require.NoError(t, err)
		require.NoError(t, db.AddPrecompileFn(db.MultiStoreSnapshot()))
		require.NoError(t, db.FlushToCacheCtx())
		bh := factory.NewBalanceHandler()
		bh.BeforeBalanceChange(cacheCtx)
		return cacheCtx, bh
	}
	transfer := func(cacheCtx sdk.Context, amt int64) {
		coins := sdk.NewCoins(sdk.NewInt64Coin(evmtypes.GetEVMCoinDenom(), amt))
		cacheCtx.EventManager().EmitEvents(sdk.Events{
			banktypes.NewCoinSpentEvent(spenderAcc, coins),
			banktypes.NewCoinReceivedEvent(receiverAcc, coins),
		})
	}

	// outer precompile action
	outerCtx, outerBH := enterPrecompile()
	transfer(outerCtx, 30)

	// nested EVM call runs an inner precompile
	innerCtx, innerBH := enterPrecompile()
	transfer(innerCtx, 10)
	require.NoError(t, innerBH.AfterBalanceChange(innerCtx, db))
	require.Equal(t, uint64(90), db.GetBalance(spender).Uint64())
	// nested EVM call returns (callFromPrecompile flush)
	require.NoError(t, db.FlushToCacheCtx())

	require.NoError(t, outerBH.AfterBalanceChange(outerCtx, db))
	require.Equal(t, uint64(60), db.GetBalance(spender).Uint64(), "spender")
	require.Equal(t, uint64(40), db.GetBalance(receiver).Uint64(), "receiver")
}
