package snapshotmulti_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm/x/vm/store/snapshotmulti"

	"cosmossdk.io/store/cachekv"
	"cosmossdk.io/store/dbadapter"
	storetypes "cosmossdk.io/store/types"
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

func TestSnapshotMultiCacheWrapWithTrace(t *testing.T) {
	snapshotStore, _ := setupStore()

	wrap := snapshotStore.CacheWrapWithTrace(nil, nil)
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

func TestSnapshotMultiMetadata(t *testing.T) {
	snapshotStore, _ := setupStore()

	require.Equal(t, storetypes.StoreTypeMulti, snapshotStore.GetStoreType())
	require.False(t, snapshotStore.TracingEnabled())
	require.Equal(t, snapshotStore, snapshotStore.SetTracer(nil))
	require.Equal(t, snapshotStore, snapshotStore.SetTracingContext(nil))
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
