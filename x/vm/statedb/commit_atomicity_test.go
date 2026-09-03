package statedb_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/vm/statedb"

	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// atomicTestKeeper routes writes through ctx's real KVStore, unlike the
// in-memory-map mocks elsewhere in this package, so a write discarded via
// CacheContext is actually observable as absent.
type atomicTestKeeper struct {
	key     *storetypes.KVStoreKey
	errAddr common.Address
}

var _ statedb.Keeper = &atomicTestKeeper{}

func (k *atomicTestKeeper) store(ctx sdk.Context) storetypes.KVStore { return ctx.KVStore(k.key) }

func (k *atomicTestKeeper) GetAccount(ctx sdk.Context, addr common.Address) *statedb.Account {
	bz := k.store(ctx).Get(addr.Bytes())
	if bz == nil {
		return nil
	}
	return &statedb.Account{Balance: new(uint256.Int).SetBytes(bz)}
}

func (k *atomicTestKeeper) SetAccount(ctx sdk.Context, addr common.Address, acc statedb.Account) error {
	if addr == k.errAddr {
		return errors.New("blocked")
	}
	k.store(ctx).Set(addr.Bytes(), acc.Balance.Bytes())
	return nil
}

func (k *atomicTestKeeper) DeleteAccount(ctx sdk.Context, addr common.Address) error {
	if addr == k.errAddr {
		return errors.New("blocked")
	}
	k.store(ctx).Delete(addr.Bytes())
	return nil
}

func (k *atomicTestKeeper) GetState(sdk.Context, common.Address, common.Hash) common.Hash {
	return common.Hash{}
}
func (k *atomicTestKeeper) GetCode(sdk.Context, common.Hash) []byte             { return nil }
func (k *atomicTestKeeper) GetCodeHash(sdk.Context, common.Address) common.Hash { return common.Hash{} }
func (k *atomicTestKeeper) ForEachStorage(sdk.Context, common.Address, func(common.Hash, common.Hash) bool) {
}
func (k *atomicTestKeeper) DeleteState(sdk.Context, common.Address, common.Hash)      {}
func (k *atomicTestKeeper) SetState(sdk.Context, common.Address, common.Hash, []byte) {}
func (k *atomicTestKeeper) DeleteCode(sdk.Context, []byte)                            {}
func (k *atomicTestKeeper) SetCode(sdk.Context, []byte, []byte)                       {}

func (k *atomicTestKeeper) KVStoreKeys() map[string]*storetypes.KVStoreKey {
	return map[string]*storetypes.KVStoreKey{k.key.Name(): k.key}
}

// TestCommitAtomicity commits a dirty set sorted [credit, blocked, debit]. A
// late failure on blocked must discard the whole commit, including credit,
// which a non-atomic commit would already have written.
func TestCommitAtomicity(t *testing.T) {
	credit := common.BigToAddress(big.NewInt(10))
	blocked := common.BigToAddress(big.NewInt(50))
	debit := common.BigToAddress(big.NewInt(90))
	precompileAddr := common.BigToAddress(big.NewInt(1)) // written via cacheCtx, bypassing the journal

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
	// persisted reads through db's own keeper/ctx, bypassing db's in-memory
	// cache, to check what actually reached the real store.
	persisted := func(db *statedb.StateDB, addr common.Address) *statedb.Account {
		return db.Keeper().GetAccount(db.GetContext(), addr)
	}
	// stageViaPrecompile writes directly through the cache context the way a
	// real precompile does, bypassing the journal entirely.
	stageViaPrecompile := func(t *testing.T, db *statedb.StateDB) {
		t.Helper()
		cacheCtx, err := db.GetCacheContext()
		require.NoError(t, err)
		require.NoError(t, db.Keeper().SetAccount(cacheCtx, precompileAddr, statedb.Account{Balance: uint256.NewInt(7)}))
	}

	t.Run("late failure discards the whole commit", func(t *testing.T) {
		db := setup("fail", blocked)
		seed(db)
		require.Error(t, db.Commit())
		require.Nil(t, persisted(db, credit))
	})

	t.Run("late failure discards precompile-staged writes too", func(t *testing.T) {
		db := setup("fail_precompile", blocked)
		stageViaPrecompile(t, db)
		seed(db)
		require.Error(t, db.Commit())
		require.Nil(t, persisted(db, credit))
		require.Nil(t, persisted(db, precompileAddr))
	})

	t.Run("success still persists everything", func(t *testing.T) {
		db := setup("ok", common.Address{})
		seed(db)
		require.NoError(t, db.Commit())
		require.Equal(t, uint256.NewInt(1_000_000), persisted(db, credit).Balance)
		require.Equal(t, uint256.NewInt(1), persisted(db, debit).Balance)
	})

	t.Run("success persists precompile-staged writes too", func(t *testing.T) {
		db := setup("ok_precompile", common.Address{})
		stageViaPrecompile(t, db)
		seed(db)
		require.NoError(t, db.Commit())
		require.Equal(t, uint256.NewInt(1_000_000), persisted(db, credit).Balance)
		require.Equal(t, uint256.NewInt(7), persisted(db, precompileAddr).Balance)
	})
}
