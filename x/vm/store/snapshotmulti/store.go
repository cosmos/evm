package snapshotmulti

import (
	"fmt"
	"io"
	"maps"

	"github.com/cosmos/evm/x/vm/store/snapshotkv"
	"github.com/cosmos/evm/x/vm/store/types"

	storecachekv "cosmossdk.io/store/cachekv"
	storetypes "cosmossdk.io/store/types"
)

type Store struct {
	stacks map[storetypes.StoreKey]types.SnapshotStack

	traceWriter  io.Writer
	traceContext storetypes.TraceContext
}

var _ types.SnapshotMultiStore = Store{}

func NewStore(stores map[storetypes.StoreKey]storetypes.CacheWrap) *Store {
	cms := &Store{
		stacks: make(map[storetypes.StoreKey]types.SnapshotStack),
	}

	for key, store := range stores {
		cms.stacks[key] = snapshotkv.NewStore(store.(*storecachekv.Store))
	}

	return cms
}

func (cms Store) Snapshot() int {
	var snapshot int
	for k := range cms.stacks {
		snapshot = cms.stacks[k].Snapshot()
	}
	return snapshot
}

func (cms Store) RevertToSnapshot(target int) {
	for _, cacheStack := range cms.stacks {
		cacheStack.RevertToSnapshot(target)
	}
}

func (cms Store) GetStoreType() storetypes.StoreType {
	return storetypes.StoreTypeMulti
}

func (cms Store) CacheWrap() storetypes.CacheWrap {
	return cms.CacheMultiStore().(storetypes.CacheWrap)
}

func (cms Store) CacheWrapWithTrace(_ io.Writer, _ storetypes.TraceContext) storetypes.CacheWrap {
	return cms.CacheWrap()
}

func (cms Store) CacheMultiStoreWithVersion(_ int64) (storetypes.CacheMultiStore, error) {
	panic("cannot branch cached multi-store-stack with a version")
}

// GetStore returns an underlying Store by key.
func (cms Store) GetStore(key storetypes.StoreKey) storetypes.Store {
	stack := cms.stacks[key]
	if key == nil || stack == nil {
		panic(fmt.Sprintf("kv store with key %v has not been registered in stores", key))
	}
	return stack.CurrentStore()
}

// GetKVStore returns an underlying KVStore by key.
func (cms Store) GetKVStore(key storetypes.StoreKey) storetypes.KVStore {
	stack := cms.stacks[key]
	if key == nil || stack == nil {
		panic(fmt.Sprintf("kv store with key %v has not been registered in stores", key))
	}
	return stack.CurrentStore()
}

func (cms Store) TracingEnabled() bool {
	return cms.traceWriter != nil
}

func (cms Store) SetTracer(w io.Writer) storetypes.MultiStore {
	cms.traceWriter = w
	return &cms
}

func (cms Store) SetTracingContext(tc storetypes.TraceContext) storetypes.MultiStore {
	if cms.traceContext != nil {
		maps.Copy(cms.traceContext, tc)
	} else {
		cms.traceContext = tc
	}

	return &cms
}

func (cms Store) LatestVersion() int64 {
	panic("cannot get latest version from branch cached multi-store")
}

func (cms Store) Write() {
	for k := range cms.stacks {
		cms.stacks[k].Commit()
		cms.stacks[k].CurrentStore().Write()
	}
}

func (cms Store) CacheMultiStore() storetypes.CacheMultiStore {
	cms.Snapshot()
	return &cms
}

func (cms Store) Copy() storetypes.CacheMultiStore {
	return &cms
}

func (cms Store) GetStores() map[storetypes.StoreKey]storetypes.CacheWrap {
	stores := make(map[storetypes.StoreKey]storetypes.CacheWrap, len(cms.stacks))
	for key, cacheStack := range cms.stacks {
		stores[key] = cacheStack.CurrentStore()
	}
	return stores
}
