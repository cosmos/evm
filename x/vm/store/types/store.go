package types

import (
	storetypes "cosmossdk.io/store/types"
)

const InitialHead = -1

type Snapshotter interface {
	Snapshot() int
	RevertToSnapshot(int)
}

type SnapshotKVStore interface {
	Snapshotter
	CurrentStore() storetypes.CacheKVStore
	Commit()
}

type SnapshotMultiStore interface {
	Snapshotter
	storetypes.CacheMultiStore
}
