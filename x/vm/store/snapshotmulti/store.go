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
	stores    map[storetypes.StoreKey]types.SnapshotKVStore
	storeKeys []storetypes.StoreKey // ordered keys
	head      int
}

var _ types.SnapshotMultiStore = (*Store)(nil)

// NewStore creates a new Store objectwith CacheMultiStore and KVStoreKeys
func NewStore(cms storetypes.MultiStore, keys map[string]storetypes.StoreKey) *Store {
	s := &Store{
		stores:    make(map[storetypes.StoreKey]types.SnapshotKVStore),
		storeKeys: vmtypes.SortedKVStoreKeys(keys),
		head:      types.InitialHead,
	}

	for _, key := range s.storeKeys {
		if _, ok := key.(*storetypes.KVStoreKey); ok {
			store := cms.GetKVStore(key)
			s.stores[key] = snapshotkv.NewStore(store.(storetypes.CacheWrap))
		} else {
			store := cms.GetObjKVStore(key)
			s.stores[key] = snapshotkv.NewStore(store.(storetypes.CacheWrap))
		}
	}

	return s
}

// NewStore creates a new Store object with KVStores
func NewStoreWithKVStores(stores map[*storetypes.KVStoreKey]storetypes.CacheWrap) *Store {
	s := &Store{
		stores: make(map[storetypes.StoreKey]types.SnapshotKVStore),
		head:   types.InitialHead,
	}

	for key, store := range stores {
		s.stores[key] = snapshotkv.NewStore(store.(storetypes.CacheKVStore))
		s.storeKeys = append(s.storeKeys, key)
	}

	sort.Slice(s.storeKeys, func(i, j int) bool {
		return s.storeKeys[i].Name() < s.storeKeys[j].Name()
	})

	return s
}

// Snapshot pushes a new cached context to the stack,
// and returns the index of it.
func (s *Store) Snapshot() int {
	for _, key := range s.storeKeys {
		s.stores[key].Snapshot()
	}
	s.head++

	// latest snapshot is just before head
	return s.head - 1
}

// RevertToSnapshot pops all the cached stores
// whose index is greator than or equal to target.
// The target should be snapshot index returned by `Snapshot`.
// This function panics if the index is out of bounds.
func (s *Store) RevertToSnapshot(target int) {
	for _, key := range s.storeKeys {
		s.stores[key].RevertToSnapshot(target)
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
	store := s.stores[key]
	if key == nil || store == nil {
		panic(fmt.Sprintf("kv store with key %v has not been registered in stores", key))
	}
	return store.CurrentStore().(storetypes.KVStore)
}

// GetKVStore returns an underlying KVStore by key.
func (s *Store) GetKVStore(key storetypes.StoreKey) storetypes.KVStore {
	store := s.stores[key]
	if key == nil || store == nil {
		panic(fmt.Sprintf("kv store with key %v has not been registered in stores", key))
	}
	return store.CurrentStore().(storetypes.KVStore)
}

func (s *Store) GetObjKVStore(key storetypes.StoreKey) storetypes.ObjKVStore {
	store := s.stores[key]
	if key == nil || store == nil {
		panic(fmt.Sprintf("kv store with key %v has not been registered in stores", key))
	}
	objStore, ok := store.CurrentStore().(storetypes.ObjKVStore)
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
		s.stores[key].Commit()
	}
	s.head = types.InitialHead
}
