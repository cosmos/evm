package types

import (
	storetypes "cosmossdk.io/store/types"
)

const EmptyStackIndex = -1

type Snapshotter interface {
	Snapshot() int
	RevertToSnapshot(int)
}

type SnapshotMultiStore interface {
	Snapshotter
	storetypes.CacheMultiStore
}

type SnapshotStack interface {
	Snapshotter
	CurrentStore() storetypes.CacheKVStore
	Commit()
}
