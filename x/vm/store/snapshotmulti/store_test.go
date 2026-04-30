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
	"github.com/cosmos/cosmos-sdk/testutil"
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

func TestSnapshotMultiTransientStore_RoutingAndSnapshot(t *testing.T) {
	kvKey := storetypes.NewKVStoreKey("kv")
	tKey := storetypes.NewTransientStoreKey("transient")
	oKey := storetypes.NewObjectStoreKey("object")
	tc := testutil.DefaultContextWithObjectStore(t, kvKey, tKey, oKey)
	cacheCtx, _ := tc.Ctx.CacheContext()

	keys := map[string]storetypes.StoreKey{
		kvKey.Name(): kvKey,
		tKey.Name():  tKey,
	}
	snap := snapshotmulti.NewStore(cacheCtx.MultiStore(), keys)

	kv := snap.GetKVStore(kvKey)
	tv := snap.GetKVStore(tKey)
	require.NotNil(t, kv)
	require.NotNil(t, tv)

	kv.Set([]byte("k"), []byte("kv-1"))
	tv.Set([]byte("k"), []byte("tx-1"))

	idx := snap.Snapshot()
	snap.GetKVStore(kvKey).Set([]byte("k"), []byte("kv-2"))
	snap.GetKVStore(tKey).Set([]byte("k"), []byte("tx-2"))

	snap.RevertToSnapshot(idx)
	require.Equal(t, []byte("kv-1"), snap.GetKVStore(kvKey).Get([]byte("k")))
	require.Equal(t, []byte("tx-1"), snap.GetKVStore(tKey).Get([]byte("k")))
}

func TestSnapshotMultiTransientStore_NotObjectStore(t *testing.T) {
	kvKey := storetypes.NewKVStoreKey("kv")
	tKey := storetypes.NewTransientStoreKey("transient")
	oKey := storetypes.NewObjectStoreKey("object")
	tc := testutil.DefaultContextWithObjectStore(t, kvKey, tKey, oKey)
	cacheCtx, _ := tc.Ctx.CacheContext()

	keys := map[string]storetypes.StoreKey{
		tKey.Name(): tKey,
	}
	snap := snapshotmulti.NewStore(cacheCtx.MultiStore(), keys)

	require.Panics(t, func() { snap.GetObjKVStore(tKey) })
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
