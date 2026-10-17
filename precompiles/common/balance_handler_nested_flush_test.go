package common_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
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

// recordingKeeper records SetAccount writes so the test can observe the
// absolute balances a flush writes to the keeper.
type recordingKeeper struct {
	*mocks.EVMKeeper
	record bool
	writes map[common.Address][]uint64
}

func (k *recordingKeeper) SetAccount(ctx sdk.Context, addr common.Address, account statedb.Account) error {
	if k.record && account.Balance != nil {
		k.writes[addr] = append(k.writes[addr], account.Balance.Uint64())
	}
	return k.EVMKeeper.SetAccount(ctx, addr, account)
}

// TestBalanceHandlerNestedFlush checks the property: after a precompile
// action returns, StateDB.GetBalance reflects every EVM-denom bank movement
// made during that action, including when the action performs a nested EVM
// call on the same StateDB (which flushes via FlushToCacheCtx, as
// ApplyMessageWithConfig does with callFromPrecompile=true) after the bank
// movement and before AfterBalanceChange runs.
//
// It also checks that no keeper write made after the bank movement (by the
// nested flush or by the final Commit) sets the spender to a balance other
// than its post-movement balance. Keeper.SetAccount sets the bank balance to
// the written value (mint/burn the difference), so such a write would
// override the bank movement.
func TestBalanceHandlerNestedFlush(t *testing.T) {
	const (
		initial  = 100
		transfer = 30
	)

	testCases := []struct {
		name string
		// spenderDirty: the spender's balance was modified through the
		// StateDB earlier in the tx (so it is in the journal's dirty set).
		spenderDirty bool
		nestedFlush  bool
	}{
		{name: "no nested flush, spender dirty", spenderDirty: true, nestedFlush: false},
		{name: "no nested flush, spender clean", spenderDirty: false, nestedFlush: false},
		{name: "nested flush, spender dirty", spenderDirty: true, nestedFlush: true},
		{name: "nested flush, spender clean", spenderDirty: false, nestedFlush: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupBalanceHandlerTest(t)

			storeKey := storetypes.NewKVStoreKey("test")
			tKey := storetypes.NewTransientStoreKey("test_t")
			ctx := sdktestutil.DefaultContext(storeKey, tKey)

			keeper := &recordingKeeper{EVMKeeper: mocks.NewEVMKeeper(), writes: map[common.Address][]uint64{}}

			_, addrs, err := testutil.GeneratePrivKeyAddressPairs(2)
			require.NoError(t, err)
			spenderAcc, receiverAcc := addrs[0], addrs[1]
			spender := common.BytesToAddress(spenderAcc)
			receiver := common.BytesToAddress(receiverAcc)

			if !tc.spenderDirty {
				acct := statedb.NewEmptyAccount()
				acct.Balance = uint256.NewInt(initial)
				require.NoError(t, keeper.SetAccount(ctx, spender, *acct))
			}

			db := statedb.New(ctx, keeper, statedb.NewEmptyTxConfig())
			if tc.spenderDirty {
				db.AddBalance(spender, uint256.NewInt(initial), tracing.BalanceChangeUnspecified)
			}
			// load both accounts into the StateDB
			require.Equal(t, uint64(initial), db.GetBalance(spender).Uint64())
			require.Equal(t, uint64(0), db.GetBalance(receiver).Uint64())

			// precompile entry, as in Precompile.runNativeAction
			cacheCtx, err := db.GetCacheContext()
			require.NoError(t, err)
			snapshot := db.MultiStoreSnapshot()
			require.NoError(t, db.AddPrecompileFn(snapshot))
			require.NoError(t, db.FlushToCacheCtx())

			bankKeeper := cmnmocks.NewBankKeeper(t)
			bankKeeper.Mock.On("BlockedAddr", mock.AnythingOfType("types.AccAddress")).Return(false).Maybe()
			bh := cmn.NewBalanceHandlerFactory(bankKeeper).NewBalanceHandler()
			bh.BeforeBalanceChange(cacheCtx)

			// bank movement of the EVM denom inside the precompile action
			coins := sdk.NewCoins(sdk.NewInt64Coin(evmtypes.GetEVMCoinDenom(), transfer))
			cacheCtx.EventManager().EmitEvents(sdk.Events{
				banktypes.NewCoinSpentEvent(spenderAcc, coins),
				banktypes.NewCoinReceivedEvent(receiverAcc, coins),
			})
			keeper.record = true

			if tc.nestedFlush {
				// nested EVM call on the same StateDB from within the action
				require.NoError(t, db.FlushToCacheCtx())
			}

			require.NoError(t, bh.AfterBalanceChange(cacheCtx, db))

			assert.Equal(t, uint64(initial-transfer), db.GetBalance(spender).Uint64(),
				"spender StateDB balance after precompile action")
			assert.Equal(t, uint64(transfer), db.GetBalance(receiver).Uint64(),
				"receiver StateDB balance after precompile action")

			require.NoError(t, db.Commit())

			// The final write (Commit) must leave the spender at its
			// post-movement balance. An intermediate nested flush can only
			// write the StateDB's view at that point, so it is not asserted.
			if ws := keeper.writes[spender]; len(ws) > 0 {
				assert.Equal(t, uint64(initial-transfer), ws[len(ws)-1],
					"spender balance in final keeper write (Commit)")
			}
		})
	}
}
