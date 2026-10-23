package statedb_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/vm/statedb"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TestFlushToCacheCtxErrorContract pins down the error contract documented on
// FlushToCacheCtx. The dirty set is sorted [credit, blocked, debit], so a
// failure on blocked happens after credit has already been written. A failed
// flush may leave the cache context torn, but nothing may reach the
// transaction context, and reverting the multistore to a snapshot taken
// before the flush must discard the partial writes.
func TestFlushToCacheCtxErrorContract(t *testing.T) {
	credit := common.BigToAddress(big.NewInt(10))
	blocked := common.BigToAddress(big.NewInt(50))
	debit := common.BigToAddress(big.NewInt(90))

	setup := func(name string, errAddr common.Address) *statedb.StateDB {
		key := storetypes.NewKVStoreKey(name)
		tkey := storetypes.NewTransientStoreKey(name + "_t")
		ctx := testutil.DefaultContext(key, tkey).WithEventManager(sdk.NewEventManager())
		return statedb.New(ctx, &atomicTestKeeper{key: key, errAddr: errAddr}, emptyTxConfig)
	}
	seed := func(db *statedb.StateDB) {
		db.AddBalance(credit, uint256.NewInt(1_000_000), tracing.BalanceChangeUnspecified)
		db.AddBalance(blocked, uint256.NewInt(1), tracing.BalanceChangeUnspecified)
		db.AddBalance(debit, uint256.NewInt(1), tracing.BalanceChangeUnspecified)
	}
	// persisted reads the transaction context's real store, bypassing both the
	// StateDB's in-memory objects and the cache context.
	persisted := func(db *statedb.StateDB, addr common.Address) *statedb.Account {
		return db.Keeper().GetAccount(db.GetContext(), addr)
	}

	t.Run("failed flush leaves cacheCtx torn but never reaches ctx", func(t *testing.T) {
		db := setup("flush_fail", blocked)
		cacheCtx, err := db.GetCacheContext()
		require.NoError(t, err)
		seed(db)

		require.Error(t, db.FlushToCacheCtx())

		// The cache context is torn: the account ordered before the failure
		// was written, the one after it was not.
		require.NotNil(t, db.Keeper().GetAccount(cacheCtx, credit))
		require.Nil(t, db.Keeper().GetAccount(cacheCtx, debit))

		// Nothing reached the transaction context.
		require.Nil(t, persisted(db, credit))
		require.Nil(t, persisted(db, debit))
	})

	t.Run("reverting to the pre-flush snapshot discards the partial writes", func(t *testing.T) {
		db := setup("flush_revert", blocked)
		cacheCtx, err := db.GetCacheContext()
		require.NoError(t, err)
		// Same order as the precompile path: snapshot, then flush.
		snapshot := db.MultiStoreSnapshot()
		seed(db)

		require.Error(t, db.FlushToCacheCtx())
		require.NotNil(t, db.Keeper().GetAccount(cacheCtx, credit))

		db.RevertMultiStore(snapshot)

		require.Nil(t, db.Keeper().GetAccount(cacheCtx, credit))
		require.Nil(t, persisted(db, credit))
	})

	t.Run("successful flush writes cacheCtx only", func(t *testing.T) {
		db := setup("flush_ok", common.Address{})
		cacheCtx, err := db.GetCacheContext()
		require.NoError(t, err)
		seed(db)

		require.NoError(t, db.FlushToCacheCtx())

		require.Equal(t, uint256.NewInt(1_000_000), db.Keeper().GetAccount(cacheCtx, credit).Balance)
		require.Equal(t, uint256.NewInt(1), db.Keeper().GetAccount(cacheCtx, debit).Balance)
		// Promotion to ctx only happens on Commit.
		require.Nil(t, persisted(db, credit))
	})
}
