package snapshotmulti

import (
	"fmt"
	"sort"

	"github.com/cosmos/evm/x/vm/store/snapshotkv"
	"github.com/cosmos/evm/x/vm/store/types"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
)

type Store struct {
	cms       storetypes.MultiStore
	stores    map[storetypes.StoreKey]*lazyStore
	storeKeys []storetypes.StoreKey // ordered keys
	head      int
}

// lazyStore defers creating the snapshot store and its cache layers until the
// key is accessed. Most txs touch a few of the ~35 stores, so eager branching wastes work.
type lazyStore struct {
	kv *snapshotkv.Store
	// levels[i] is the multi store snapshot index that kv layer i belongs to.
	levels []int
}

var _ types.SnapshotMultiStore = (*Store)(nil)

// NewStore creates a new Store objectwith CacheMultiStore and KVStoreKeys
func NewStore(cms storetypes.MultiStore, keys map[string]storetypes.StoreKey) *Store {
	s := &Store{
		cms:       cms,
		stores:    make(map[storetypes.StoreKey]*lazyStore, len(keys)),
		storeKeys: vmtypes.SortedKVStoreKeys(keys),
		head:      types.InitialHead,
	}

	for _, key := range s.storeKeys {
		s.stores[key] = &lazyStore{}
	}

	return s
}

// NewStore creates a new Store object with KVStores
func NewStoreWithKVStores(stores map[*storetypes.KVStoreKey]storetypes.CacheWrap) *Store {
	s := &Store{
		stores: make(map[storetypes.StoreKey]*lazyStore),
		head:   types.InitialHead,
	}

	for key, store := range stores {
		s.stores[key] = &lazyStore{kv: snapshotkv.NewStore(store.(storetypes.CacheKVStore))}
		s.storeKeys = append(s.storeKeys, key)
	}

	sort.Slice(s.storeKeys, func(i, j int) bool {
		return s.storeKeys[i].Name() < s.storeKeys[j].Name()
	})

	return s
}

// current returns the active store for key, creating the snapshot store and
// the cache layer for the current snapshot on first access.
func (s *Store) current(key storetypes.StoreKey) storetypes.CacheWrap {
	ls := s.stores[key]
	if key == nil || ls == nil {
		panic(fmt.Sprintf("kv store with key %v has not been registered in stores", key))
	}
	if ls.kv == nil {
		var store any
		if _, ok := key.(*storetypes.KVStoreKey); ok {
			store = s.cms.GetKVStore(key)
		} else {
			store = s.cms.GetObjKVStore(key)
		}
		ls.kv = snapshotkv.NewStore(store.(storetypes.CacheWrap))
	}
	// Layers skipped for snapshots where the key wasn't touched hold no writes,
	// so a single layer for the latest snapshot is equivalent.
	if top := s.head - 1; top >= 0 && (len(ls.levels) == 0 || ls.levels[len(ls.levels)-1] != top) {
		ls.kv.Snapshot()
		ls.levels = append(ls.levels, top)
	}
	return ls.kv.CurrentStore()
}

// Snapshot pushes a new cached context to the stack,
// and returns the index of it.
func (s *Store) Snapshot() int {
	s.head++

	// latest snapshot is just before head
	return s.head - 1
}

// RevertToSnapshot pops all the cached stores
// whose index is greator than or equal to target.
// The target should be snapshot index returned by `Snapshot`.
// This function panics if the index is out of bounds.
func (s *Store) RevertToSnapshot(target int) {
	if target < 0 || target >= s.head {
		panic(fmt.Errorf("snapshot index %d out of bound [%d..%d)", target, 0, s.head))
	}
	for _, ls := range s.stores {
		n := len(ls.levels)
		for n > 0 && ls.levels[n-1] >= target {
			n--
		}
		if n < len(ls.levels) {
			ls.kv.RevertToSnapshot(n)
			ls.levels = ls.levels[:n]
		}
	}
	s.head = target
}

// GetStoreType returns the type of the store.
func (s *Store) GetStoreType() storetypes.StoreType {
	return storetypes.StoreTypeMulti
}

// Implements CacheWrapper.
func (s *Store) CacheWrap() storetypes.CacheWrap {
	return s.CacheMultiStore().(storetypes.CacheWrap)
}

// CacheMultiStore snapshots store and return current store.
func (s *Store) CacheMultiStore() storetypes.CacheMultiStore {
	s.Snapshot()
	return s
}

// CacheMultiStoreWithVersion load stores at a snapshot version.
//
// NOTE: CacheMultiStoreWithVersion is no-op function.
func (s *Store) CacheMultiStoreWithVersion(_ int64) (storetypes.CacheMultiStore, error) {
	return s, nil
}

// GetStore returns an underlying Store by key.
func (s *Store) GetStore(key storetypes.StoreKey) storetypes.Store {
	return s.current(key).(storetypes.KVStore)
}

// GetKVStore returns an underlying KVStore by key.
func (s *Store) GetKVStore(key storetypes.StoreKey) storetypes.KVStore {
	return s.current(key).(storetypes.KVStore)
}

func (s *Store) GetObjKVStore(key storetypes.StoreKey) storetypes.ObjKVStore {
	objStore, ok := s.current(key).(storetypes.ObjKVStore)
	if !ok {
		panic(fmt.Sprintf("store with key %v is not ObjKVStore", key))
	}
	return objStore
}

// LatestVersion returns the branch version of the store
func (s *Store) LatestVersion() int64 {
	return int64(s.head)
}

// Write calls Write on each underlying store.
func (s *Store) Write() {
	for _, key := range s.storeKeys {
		ls := s.stores[key]
		if ls.kv == nil {
			continue
		}
		ls.kv.Commit()
		ls.levels = nil
	}
	s.head = types.InitialHead
}
