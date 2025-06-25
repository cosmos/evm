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
	cms, _ := setupStore()

	idx0 := cms.Snapshot()
	require.Equal(t, 0, idx0)

	idx1 := cms.Snapshot()
	require.Equal(t, 1, idx1)

	idx2 := cms.Snapshot()
	require.Equal(t, 2, idx2)
}

func TestSnapshotMultiRevertAndWrite(t *testing.T) {
	cms, key := setupStore()
	kv := cms.GetKVStore(key)
	kv.Set([]byte("a"), []byte("1"))

	idx0 := cms.Snapshot()
	cms.GetKVStore(key).Set([]byte("b"), []byte("2"))

	idx1 := cms.Snapshot()
	cms.GetKVStore(key).Set([]byte("c"), []byte("3"))

	cms.RevertToSnapshot(idx1)
	require.Nil(t, cms.GetKVStore(key).Get([]byte("c")))
	require.Equal(t, []byte("2"), cms.GetKVStore(key).Get([]byte("b")))

	cms.RevertToSnapshot(idx0)
	require.Nil(t, cms.GetKVStore(key).Get([]byte("b")))
	require.Equal(t, []byte("1"), cms.GetKVStore(key).Get([]byte("a")))

	cms.Snapshot()
	cms.GetKVStore(key).Set([]byte("d"), []byte("4"))
	cms.Write()

	require.Equal(t, []byte("4"), kv.Get([]byte("d")))
	idx := cms.Snapshot()
	require.Equal(t, 0, idx)
}

func TestSnapshotMultiInvalidIndex(t *testing.T) {
	cms, _ := setupStore()
	cms.Snapshot()

	require.PanicsWithError(t, "snapshot index 1 out of bound [0..1)", func() {
		cms.RevertToSnapshot(1)
	})

	require.PanicsWithError(t, "snapshot index -1 out of bound [0..1)", func() {
		cms.RevertToSnapshot(-1)
	})
}

func TestSnapshotMultiGetStore(t *testing.T) {
	cms, key := setupStore()

	s := cms.GetStore(key)
	require.NotNil(t, s)
	require.Equal(t, cms.GetKVStore(key), s)

	badKey := storetypes.NewKVStoreKey("bad")
	require.Panics(t, func() { cms.GetStore(badKey) })
	require.Panics(t, func() { cms.GetKVStore(badKey) })
}

func TestSnapshotMultiCacheWrap(t *testing.T) {
	cms, _ := setupStore()

	wrap := cms.CacheWrap()
	require.Equal(t, cms, wrap)

	idx := cms.Snapshot()
	require.Equal(t, 1, idx)
}

func TestSnapshotMultiCacheWrapWithTrace(t *testing.T) {
	cms, _ := setupStore()

	wrap := cms.CacheWrapWithTrace(nil, nil)
	require.Equal(t, cms, wrap)

	idx := cms.Snapshot()
	require.Equal(t, 1, idx)
}

func TestSnapshotMultiCacheMultiStore(t *testing.T) {
	cms, _ := setupStore()

	m := cms.CacheMultiStore()
	require.Equal(t, cms, m)

	idx := cms.Snapshot()
	require.Equal(t, 1, idx)
}

func TestSnapshotMultiCacheMultiStoreWithVersion(t *testing.T) {
	cms, _ := setupStore()

	m, _ := cms.CacheMultiStoreWithVersion(1)
	require.Equal(t, cms, m)
}

func TestSnapshotMultiMetadata(t *testing.T) {
	cms, _ := setupStore()

	require.Equal(t, storetypes.StoreTypeMulti, cms.GetStoreType())
	require.False(t, cms.TracingEnabled())
	require.Equal(t, cms, cms.SetTracer(nil))
	require.Equal(t, cms, cms.SetTracingContext(nil))
}

func TestSnapshotMultiLatestVersion(t *testing.T) {
	cms, _ := setupStore()

	initialVersion := int64(0)
	ver0 := cms.LatestVersion()
	require.Equal(t, ver0, initialVersion)

	idx0 := cms.Snapshot()
	ver1 := cms.LatestVersion()
	require.Equal(t, ver1, int64(idx0+1))

	idx1 := cms.Snapshot()
	ver2 := cms.LatestVersion()
	require.Equal(t, ver2, int64(idx1+1))
}
