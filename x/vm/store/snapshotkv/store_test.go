package snapshotkv_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm/x/vm/store/snapshotkv"

	"cosmossdk.io/store/cachekv"
	"cosmossdk.io/store/dbadapter"
	storetypes "cosmossdk.io/store/types"
)

func newSnapshotKV() *snapshotkv.Store {
	base := cachekv.NewStore(dbadapter.Store{DB: dbm.NewMemDB()})
	return snapshotkv.NewStore(base)
}

func TestSnapshotIndexing(t *testing.T) {
	store := newSnapshotKV()

	idx0 := store.Snapshot()
	require.Equal(t, 0, idx0)

	idx1 := store.Snapshot()
	require.Equal(t, 1, idx1)

	idx2 := store.Snapshot()
	require.Equal(t, 2, idx2)
}

func TestSnapshotRevertAndCommit(t *testing.T) {
	store := newSnapshotKV()

	// set in base store
	base := store.CurrentStore().(storetypes.KVStore)
	base.Set([]byte("a"), []byte("1"))

	idx0 := store.Snapshot()
	store.CurrentStore().(storetypes.KVStore).Set([]byte("b"), []byte("2"))

	idx1 := store.Snapshot()
	store.CurrentStore().(storetypes.KVStore).Set([]byte("c"), []byte("3"))

	// revert latest snapshot (idx1)
	store.RevertToSnapshot(idx1)
	require.Nil(t, store.CurrentStore().(storetypes.KVStore).Get([]byte("c")))
	require.Equal(t, []byte("2"), store.CurrentStore().(storetypes.KVStore).Get([]byte("b")))

	// revert the first snapshot
	store.RevertToSnapshot(idx0)
	require.Nil(t, store.CurrentStore().(storetypes.KVStore).Get([]byte("b")))
	require.Equal(t, []byte("1"), store.CurrentStore().(storetypes.KVStore).Get([]byte("a")))

	// take new snapshot and commit
	store.Snapshot()
	store.CurrentStore().(storetypes.KVStore).Set([]byte("d"), []byte("4"))
	store.Commit()

	require.Equal(t, []byte("4"), base.Get([]byte("d")))

	// commit clears the snapshot stack
	idx := store.Snapshot()
	require.Equal(t, 0, idx)
}

func TestSnapshotKVRevertOverwriteSameKey(t *testing.T) {
	// Initialize a fresh SnapshotKVStore
	store := newSnapshotKV()
	base := store.CurrentStore()

	// Initial write under key "a"
	store.CurrentStore().(storetypes.KVStore).Set([]byte("a"), []byte("1"))

	// Overwrite "a" with "2"
	idx0 := store.Snapshot()
	store.CurrentStore().(storetypes.KVStore).Set([]byte("a"), []byte("2"))

	// Overwrite "a" with "3"
	idx1 := store.Snapshot()
	store.CurrentStore().(storetypes.KVStore).Set([]byte("a"), []byte("3"))

	// Revert to idx1: expect value "2"
	store.RevertToSnapshot(idx1)
	require.Equal(t, []byte("2"), store.CurrentStore().(storetypes.KVStore).Get([]byte("a")))

	// Revert to idx0: expect value "1"
	store.RevertToSnapshot(idx0)
	require.Equal(t, []byte("1"), store.CurrentStore().(storetypes.KVStore).Get([]byte("a")))

	// Take a new snapshot, overwrite "a" with "4", then commit
	idx2 := store.Snapshot()
	store.CurrentStore().(storetypes.KVStore).Set([]byte("a"), []byte("4"))
	store.Commit()

	// After commit, the base store should have "4"
	require.Equal(t, []byte("4"), base.(storetypes.KVStore).Get([]byte("a")))

	// Commit clears the snapshot stack, so reverting to idx2 should panic
	expectedErr := fmt.Sprintf("snapshot index %d out of bound [%d..%d)", idx2, 0, 0)
	require.PanicsWithErrorf(
		t,
		expectedErr,
		func() { store.RevertToSnapshot(idx2) },
		"RevertToSnapshot should panic when idx out of bounds",
	)
}

func TestSnapshotInvalidIndex(t *testing.T) {
	store := newSnapshotKV()
	store.Snapshot()

	require.PanicsWithError(t, "snapshot index 1 out of bound [0..1)", func() {
		store.RevertToSnapshot(1)
	})

	require.PanicsWithError(t, "snapshot index -1 out of bound [0..1)", func() {
		store.RevertToSnapshot(-1)
	})
}
