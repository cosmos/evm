package snapshotkv

import (
	"fmt"

	"cosmossdk.io/store/cachekv"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/evm/x/vm/store/types"
)

// CacheKVStack manages a stack of nested cache store to
// support the evm `StateDB`'s `Snapshot` and `RevertToSnapshot` methods.
type Store struct {
	initialStore storetypes.CacheKVStore
	// Context of the initial state before transaction execution.
	// It's the context used by `StateDB.CommitedState`.
	cacheStores []storetypes.CacheKVStore
}

var _ types.SnapshotStack = (*Store)(nil)

// NewStore creates a new Store object
func NewStore(store storetypes.CacheKVStore) *Store {
	return &Store{
		initialStore: store,
		cacheStores:  nil,
	}
}

// CurrentContext returns the top context of cached stack,
// if the stack is empty, returns the initial context.
func (cs *Store) CurrentStore() storetypes.CacheKVStore {
	l := len(cs.cacheStores)
	if l == 0 {
		return cs.initialStore
	}
	return cs.cacheStores[l-1]
}

// Commit commits all the cached contexts from top to bottom in order and clears the stack by setting an empty slice of cache contexts.
func (cs *Store) Commit() {
	// commit in order from top to bottom
	for i := len(cs.cacheStores) - 1; i >= 0; i-- {
		cs.cacheStores[i].Write()
	}
	cs.cacheStores = nil
}

// Snapshot pushes a new cached context to the stack,
// and returns the index of it.
func (cs *Store) Snapshot() int {
	cs.cacheStores = append(cs.cacheStores, cachekv.NewStore(cs.CurrentStore()))
	return len(cs.cacheStores) - 1
}

// RevertToSnapshot pops all the cached contexts after the target index (inclusive).
// the target should be snapshot index returned by `Snapshot`.
// This function panics if the index is out of bounds.
func (cs *Store) RevertToSnapshot(target int) {
	if target < 0 || target >= len(cs.cacheStores) {
		panic(fmt.Errorf("snapshot index %d out of bound [%d..%d)", target, 0, len(cs.cacheStores)))
	}
	cs.cacheStores = cs.cacheStores[:target]
}
