package snapshotkv

import (
	"fmt"

	"github.com/cosmos/evm/x/vm/store/types"

	storetypes "cosmossdk.io/store/types"
)

// Store manages a stack of nested cache store to
// support the evm `StateDB`'s `Snapshot` and `RevertToSnapshot` methods.
type Store struct {
	// Store of the initial state before transaction execution
	initialStore storetypes.CacheWrap

	// Stack of cached store
	cacheStores []storetypes.CacheWrap
}

var _ types.SnapshotKVStore = (*Store)(nil)

// NewStore creates a new Store object
func NewStore(store storetypes.CacheWrap) *Store {
	return &Store{
		initialStore: store,
		cacheStores:  nil,
	}
}

// CurrentStore returns the top of cached store stack.
// If the stack is empty, returns the initial store.
func (cs *Store) CurrentStore() storetypes.CacheWrap {
	l := len(cs.cacheStores)
	if l == 0 {
		return cs.initialStore
	}
	return cs.cacheStores[l-1]
}

// Commit commits all the cached stores from top to bottom in order
// and clears the cache stack by setting an empty slice of cache store.
func (cs *Store) Commit() {
	// commit in order from top to bottom
	for i := len(cs.cacheStores) - 1; i >= 0; i-- {
		cs.cacheStores[i].Write()
	}
	cs.initialStore.Write()
	cs.cacheStores = nil
}

// Snapshot pushes a new cached store to the stack,
// and returns the index of it.
func (cs *Store) Snapshot() int {
	cs.cacheStores = append(cs.cacheStores, cs.CurrentStore().CacheWrap())
	return len(cs.cacheStores) - 1
}

// RevertToSnapshot pops all the cached stores
// whose index is greator than or equal to target.
// The target should be snapshot index returned by `Snapshot`.
// This function panics if the index is out of bounds.
func (cs *Store) RevertToSnapshot(target int) {
	if target < 0 || target >= len(cs.cacheStores) {
		panic(fmt.Errorf("snapshot index %d out of bound [%d..%d)", target, 0, len(cs.cacheStores)))
	}
	cs.cacheStores = cs.cacheStores[:target]
}
