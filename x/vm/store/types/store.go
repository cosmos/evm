package types

import (
	storetypes "cosmossdk.io/store/types"
)

const InitialHead = 0

// Snapshotter defines behavior for taking and reverting to state snapshots.
type Snapshotter interface {
	// Snapshot captures the current state and returns a snapshot identifier.
	// The returned int can be used later to revert back to this exact state.
	Snapshot() int

	// RevertToSnapshot rolls back the state to the snapshot corresponding
	// to the given identifier. All changes made after that snapshot will be discarded.
	RevertToSnapshot(int)
}

// SnapshotKVStore extends Snapshotter with CacheKVStore-specific operations.
//
// It allows you to take/revert snapshots around KV-store operations,
// inspect the current active store, and commit changes.
type SnapshotKVStore interface {
	Snapshotter

	// CurrentStore returns the underlying CacheKVStore that is currently
	// active (i.e., where reads and writes will be applied).
	CurrentStore() storetypes.CacheWrap

	// Commit flushes all pending changes in the current store layer
	// down to its parent, making them permanent.
	Commit()
}

// SnapshotMultiStore extends Snapshotter and CacheMultiStore.
//
// It allows snapshotting and rollback semantics on a multi-store
// (i.e., a collection of keyed sub-stores), leveraging the existing
// CacheMultiStore interface for basic store and cache management.
type SnapshotMultiStore interface {
	Snapshotter
	storetypes.CacheMultiStore
}
