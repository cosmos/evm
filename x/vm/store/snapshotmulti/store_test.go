package snapshotmulti_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm/x/vm/store/snapshotmulti"

	"github.com/cosmos/cosmos-sdk/store/v2/cachekv"
	"github.com/cosmos/cosmos-sdk/store/v2/dbadapter"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
)

func setupStore() (*snapshotmulti.Store, *storetypes.KVStoreKey) {
	key := storetypes.NewKVStoreKey("store")
	kv := cachekv.NewStore(dbadapter.Store{DB: dbm.NewMemDB()})
	stores := map[*storetypes.KVStoreKey]storetypes.CacheWrap{key: kv}
	return snapshotmulti.NewStoreWithKVStores(stores), key
}

func TestSnapshotMultiIndexing(t *testing.T) {
	snapshotStore, _ := setupStore()

	idx0 := snapshotStore.Snapshot()
	require.Equal(t, 0, idx0)

	idx1 := snapshotStore.Snapshot()
	require.Equal(t, 1, idx1)

	idx2 := snapshotStore.Snapshot()
	require.Equal(t, 2, idx2)
}

func TestSnapshotMultiRevertAndWrite(t *testing.T) {
	snapshotStore, key := setupStore()
	kv := snapshotStore.GetKVStore(key)
	kv.Set([]byte("a"), []byte("1"))

	idx0 := snapshotStore.Snapshot()
	snapshotStore.GetKVStore(key).Set([]byte("b"), []byte("2"))

	idx1 := snapshotStore.Snapshot()
	snapshotStore.GetKVStore(key).Set([]byte("c"), []byte("3"))

	snapshotStore.RevertToSnapshot(idx1)
	require.Nil(t, snapshotStore.GetKVStore(key).Get([]byte("c")))
	require.Equal(t, []byte("2"), snapshotStore.GetKVStore(key).Get([]byte("b")))

	snapshotStore.RevertToSnapshot(idx0)
	require.Nil(t, snapshotStore.GetKVStore(key).Get([]byte("b")))
	require.Equal(t, []byte("1"), snapshotStore.GetKVStore(key).Get([]byte("a")))

	snapshotStore.Snapshot()
	snapshotStore.GetKVStore(key).Set([]byte("d"), []byte("4"))
	snapshotStore.Write()

	require.Equal(t, []byte("4"), kv.Get([]byte("d")))
	idx := snapshotStore.Snapshot()
	require.Equal(t, 0, idx)
}

func TestSnapshotMultiRevertOverwriteSameKey(t *testing.T) {
	// Setup a fresh SnapshotMultiStore and a key
	snapshotStore, key := setupStore()
	kv := snapshotStore.GetKVStore(key)

	// Initial write under key "a"
	kv.Set([]byte("a"), []byte("1"))

	// Overwrite "a" with "2"
	idx0 := snapshotStore.Snapshot()
	snapshotStore.GetKVStore(key).Set([]byte("a"), []byte("2"))

	// Overwrite "a" with "3"
	idx1 := snapshotStore.Snapshot()
	snapshotStore.GetKVStore(key).Set([]byte("a"), []byte("3"))

	// Revert to idx1: expect value "2"
	snapshotStore.RevertToSnapshot(idx1)
	require.Equal(t, []byte("2"), snapshotStore.GetKVStore(key).Get([]byte("a")))

	// Revert to idx0: expect value "1"
	snapshotStore.RevertToSnapshot(idx0)
	require.Equal(t, []byte("1"), snapshotStore.GetKVStore(key).Get([]byte("a")))

	// Overwrite "a" with "4"
	idx2 := snapshotStore.Snapshot()
	snapshotStore.GetKVStore(key).Set([]byte("a"), []byte("4"))
	snapshotStore.Write()

	// After write, the base store should have "4"
	require.Equal(t, []byte("4"), kv.Get([]byte("a")))

	// Write clears the snapshot stack, so reverting to idx2 should panic
	expectedErr := fmt.Sprintf("snapshot index %d out of bound [%d..%d)", idx2, 0, 0)
	require.PanicsWithErrorf(t, expectedErr, func() {
		snapshotStore.RevertToSnapshot(idx2)
	}, "RevertToSnapshot should panic when idx out of bounds")
}

func TestSnapshotMultiInvalidIndex(t *testing.T) {
	snapshotStore, _ := setupStore()
	snapshotStore.Snapshot()

	require.PanicsWithError(t, "snapshot index 1 out of bound [0..1)", func() {
		snapshotStore.RevertToSnapshot(1)
	})

	require.PanicsWithError(t, "snapshot index -1 out of bound [0..1)", func() {
		snapshotStore.RevertToSnapshot(-1)
	})
}

func TestSnapshotMultiGetStore(t *testing.T) {
	snapshotStore, key := setupStore()

	s := snapshotStore.GetStore(key)
	require.NotNil(t, s)
	require.Equal(t, snapshotStore.GetKVStore(key), s)

	badKey := storetypes.NewKVStoreKey("bad")
	require.Panics(t, func() { snapshotStore.GetStore(badKey) })
	require.Panics(t, func() { snapshotStore.GetKVStore(badKey) })
}

func TestSnapshotMultiCacheWrap(t *testing.T) {
	snapshotStore, _ := setupStore()

	wrap := snapshotStore.CacheWrap()
	require.Equal(t, snapshotStore, wrap)

	idx := snapshotStore.Snapshot()
	require.Equal(t, 1, idx)
}

func TestSnapshotMultiCacheMultiStore(t *testing.T) {
	snapshotStore, _ := setupStore()

	m := snapshotStore.CacheMultiStore()
	require.Equal(t, snapshotStore, m)

	idx := snapshotStore.Snapshot()
	require.Equal(t, 1, idx)
}

func TestSnapshotMultiCacheMultiStoreWithVersion(t *testing.T) {
	snapshotStore, _ := setupStore()

	m, _ := snapshotStore.CacheMultiStoreWithVersion(1)
	require.Equal(t, snapshotStore, m)
}

func TestSnapshotMultiLatestVersion(t *testing.T) {
	snapshotStore, _ := setupStore()

	initialVersion := int64(0)
	ver0 := snapshotStore.LatestVersion()
	require.Equal(t, ver0, initialVersion)

	idx0 := snapshotStore.Snapshot()
	ver1 := snapshotStore.LatestVersion()
	require.Equal(t, ver1, int64(idx0+1))

	idx1 := snapshotStore.Snapshot()
	ver2 := snapshotStore.LatestVersion()
	require.Equal(t, ver2, int64(idx1+1))
}

// Stores touched in only some snapshots must revert as if every snapshot branched them.
func TestSnapshotMultiLazyLayers(t *testing.T) {
	t.Parallel()
	keyA, keyB := storetypes.NewKVStoreKey("a"), storetypes.NewKVStoreKey("b")
	kvA := cachekv.NewStore(dbadapter.Store{DB: dbm.NewMemDB()})
	kvB := cachekv.NewStore(dbadapter.Store{DB: dbm.NewMemDB()})
	s := snapshotmulti.NewStoreWithKVStores(map[*storetypes.KVStoreKey]storetypes.CacheWrap{keyA: kvA, keyB: kvB})
	k := []byte("k")

	s.GetKVStore(keyA).Set(k, []byte("a0")) // before any snapshot
	idx0 := s.Snapshot()
	s.GetKVStore(keyA).Set(k, []byte("a1"))
	idx1 := s.Snapshot()
	_ = s.Snapshot()
	s.GetKVStore(keyB).Set(k, []byte("b3")) // b only touched after idx2
	idx3 := s.Snapshot()
	s.GetKVStore(keyA).Set(k, []byte("a4"))

	s.RevertToSnapshot(idx3)
	require.Equal(t, []byte("a1"), s.GetKVStore(keyA).Get(k))
	require.Equal(t, []byte("b3"), s.GetKVStore(keyB).Get(k))

	s.RevertToSnapshot(idx1)
	require.Equal(t, []byte("a1"), s.GetKVStore(keyA).Get(k))
	require.Nil(t, s.GetKVStore(keyB).Get(k))

	s.GetKVStore(keyB).Set(k, []byte("b5"))
	s.RevertToSnapshot(idx0)
	require.Equal(t, []byte("a0"), s.GetKVStore(keyA).Get(k))
	require.Nil(t, s.GetKVStore(keyB).Get(k))

	s.GetKVStore(keyB).Set(k, []byte("b6"))
	s.Write()
	require.Equal(t, []byte("a0"), kvA.Get(k))
	require.Equal(t, []byte("b6"), kvB.Get(k))
}
