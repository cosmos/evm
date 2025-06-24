package snapshotmulti

import (
	"fmt"
	"io"

	"github.com/cosmos/evm/x/vm/store/snapshotkv"
	"github.com/cosmos/evm/x/vm/store/types"

	storecachekv "cosmossdk.io/store/cachekv"
	storetypes "cosmossdk.io/store/types"
)

type Store struct {
	stacks   map[storetypes.StoreKey]types.SnapshotStack
	stackIdx int

	// traceWriter  io.Writer
	// traceContext storetypes.TraceContext
}

var _ types.SnapshotMultiStore = (*Store)(nil)

// NewStore creates a new Store object
func NewStore(stores map[storetypes.StoreKey]storetypes.CacheWrap) *Store {
	cms := &Store{
		stacks:   make(map[storetypes.StoreKey]types.SnapshotStack),
		stackIdx: types.EmptyStackIndex,
	}

	for key, store := range stores {
		cms.stacks[key] = snapshotkv.NewStore(store.(*storecachekv.Store))
	}

	return cms
}

// Snapshot pushes a new cached context to the stack,
// and returns the index of it.
func (cms *Store) Snapshot() int {
	for k := range cms.stacks {
		cms.stacks[k].Snapshot()
	}
	cms.stackIdx++

	return cms.stackIdx
}

// RevertToSnapshot pops all the cached contexts after the target index (inclusive).
// the target should be snapshot index returned by `Snapshot`.
// This function panics if the index is out of bounds.
func (cms *Store) RevertToSnapshot(target int) {
	for _, cacheStack := range cms.stacks {
		cacheStack.RevertToSnapshot(target)
	}
	cms.stackIdx = target - 1
}

// GetStoreType returns the type of the store.
func (cms *Store) GetStoreType() storetypes.StoreType {
	return storetypes.StoreTypeMulti
}

// Implements CacheWrapper.
func (cms *Store) CacheWrap() storetypes.CacheWrap {
	return cms.CacheMultiStore().(storetypes.CacheWrap)
}

// CacheWrapWithTrace implements the CacheWrapper interface.
func (cms *Store) CacheWrapWithTrace(_ io.Writer, _ storetypes.TraceContext) storetypes.CacheWrap {
	return cms.CacheWrap()
}

// CacheMultiStore create cache
func (cms *Store) CacheMultiStore() storetypes.CacheMultiStore {
	cms.Snapshot()
	return cms
}

// CacheMultiStoreWithVersion implements the MultiStore interface. It will panic
// as an already cached multi-store cannot load previous versions.
//
// TODO: The store implementation can possibly be modified to support this as it
// seems safe to load previous versions (heights).
func (cms *Store) CacheMultiStoreWithVersion(_ int64) (storetypes.CacheMultiStore, error) {
	panic("cannot branch cache snapshot multi store with a version")
}

// GetStore returns an underlying Store by key.
func (cms *Store) GetStore(key storetypes.StoreKey) storetypes.Store {
	stack := cms.stacks[key]
	if key == nil || stack == nil {
		panic(fmt.Sprintf("kv store with key %v has not been registered in stores", key))
	}
	return stack.CurrentStore()
}

// GetKVStore returns an underlying KVStore by key.
func (cms *Store) GetKVStore(key storetypes.StoreKey) storetypes.KVStore {
	stack := cms.stacks[key]
	if key == nil || stack == nil {
		panic(fmt.Sprintf("kv store with key %v has not been registered in stores", key))
	}
	return stack.CurrentStore()
}

// TracingEnabled returns if tracing is enabled for the MultiStore.
//
// TODO: The store implementation can possibly be modified to support this method.
func (cms *Store) TracingEnabled() bool {
	return false
}

// SetTracer sets the tracer for the MultiStore that the underlying
// stores will utilize to trace operations. A MultiStore is returned.
//
// TODO: The store implementation can possibly be modified to support this method.
func (cms *Store) SetTracer(_ io.Writer) storetypes.MultiStore {
	return cms
}

// SetTracingContext updates the tracing context for the MultiStore by merging
// the given context with the existing context by key. Any existing keys will
// be overwritten. It is implied that the caller should update the context when
// necessary between tracing operations. It returns a modified MultiStore.
//
// TODO: The store implementation can possibly be modified to support this method.
func (cms *Store) SetTracingContext(_ storetypes.TraceContext) storetypes.MultiStore {
	return cms
}

// LatestVersion returns the branch version of the store
func (cms *Store) LatestVersion() int64 {
	panic("cannot get latest version from branch cached multi-store")
}

// Write calls Write on each underlying store.
func (cms *Store) Write() {
	for k := range cms.stacks {
		cms.stacks[k].Commit()
		cms.stacks[k].CurrentStore().Write()
	}
	cms.stackIdx = types.EmptyStackIndex
}
