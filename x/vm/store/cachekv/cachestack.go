package cachekv

import (
	"fmt"

	"cosmossdk.io/store/cachekv"

	storetypes "cosmossdk.io/store/types"
)

// CacheKVStack manages a stack of nested cache store to
// support the evm `StateDB`'s `Snapshot` and `RevertToSnapshot` methods.
type CacheKVStack struct {
	initialStore storetypes.CacheKVStore
	// Context of the initial state before transaction execution.
	// It's the context used by `StateDB.CommitedState`.
	cacheStores []storetypes.CacheKVStore
}

func NewCacheKVStack(store storetypes.CacheKVStore) *CacheKVStack {
	return &CacheKVStack{
		initialStore: store,
		cacheStores:  nil,
	}
}

// CurrentContext returns the top context of cached stack,
// if the stack is empty, returns the initial context.
func (cs *CacheKVStack) CurrentStore() storetypes.CacheKVStore {
	l := len(cs.cacheStores)
	if l == 0 {
		return cs.initialStore
	}
	return cs.cacheStores[l-1]
}

// Reset sets the initial context and clear the cache context stack.
func (cs *CacheKVStack) Reset(initialStore storetypes.CacheKVStore) {
	cs.initialStore = initialStore
	cs.cacheStores = nil
}

// IsEmpty returns true if the cache context stack is empty.
func (cs *CacheKVStack) IsEmpty() bool {
	return len(cs.cacheStores) == 0
}

// Commit commits all the cached contexts from top to bottom in order and clears the stack by setting an empty slice of cache contexts.
func (cs *CacheKVStack) Commit() {
	// commit in order from top to bottom
	for i := len(cs.cacheStores) - 1; i >= 0; i-- {
		cs.cacheStores[i].Write()
	}
	cs.cacheStores = nil
}

// CommitToRevision commit the cache after the target revision,
// to improve efficiency of db operations.
func (cs *CacheKVStack) CommitToRevision(target int) error {
	if target < 0 || target >= len(cs.cacheStores) {
		return fmt.Errorf("snapshot index %d out of bound [%d..%d)", target, 0, len(cs.cacheStores))
	}

	// commit in order from top to bottom
	for i := len(cs.cacheStores) - 1; i > target; i-- {
		cs.cacheStores[i].Write()
	}
	cs.cacheStores = cs.cacheStores[0 : target+1]

	return nil
}

// Snapshot pushes a new cached context to the stack,
// and returns the index of it.
func (cs *CacheKVStack) Snapshot() int {
	cs.cacheStores = append(cs.cacheStores, cachekv.NewStore(cs.CurrentStore()))
	return len(cs.cacheStores) - 1
}

// RevertToSnapshot pops all the cached contexts after the target index (inclusive).
// the target should be snapshot index returned by `Snapshot`.
// This function panics if the index is out of bounds.
func (cs *CacheKVStack) RevertToSnapshot(target int) {
	if target < 0 || target >= len(cs.cacheStores) {
		panic(fmt.Errorf("snapshot index %d out of bound [%d..%d)", target, 0, len(cs.cacheStores)))
	}
	cs.cacheStores = cs.cacheStores[:target]
}
