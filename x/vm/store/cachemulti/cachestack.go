package cachemulti

import (
	"fmt"

	"github.com/cosmos/evm/x/vm/store/cachekv"

	storecachekv "cosmossdk.io/store/cachekv"
	"cosmossdk.io/store/cachemulti"
	storetypes "cosmossdk.io/store/types"
)

type CacheMultiStack struct {
	cachemulti.Store
	stacks map[storetypes.StoreKey]*cachekv.CacheKVStack
}

func NewCacheMultiStack(stores map[storetypes.StoreKey]storetypes.CacheWrap) *CacheMultiStack {
	cms := &CacheMultiStack{
		stacks: make(map[storetypes.StoreKey]*cachekv.CacheKVStack),
	}

	for key, store := range stores {
		cms.stacks[key] = cachekv.NewCacheKVStack(store.(*storecachekv.Store))
	}

	return cms
}

func (cms *CacheMultiStack) Commit() {
	for k := range cms.stacks {
		cms.stacks[k].Commit()
		cms.stacks[k].CurrentStore().Write()
	}
}

// func (cms *CacheMultiStack) CurrentStore() storetypes.CacheMultiStore {
// 	stores := make(map[storetypes.StoreKey]storetypes.CacheWrapper)
// 	keys := make(map[string]storetypes.StoreKey)
// 	for key, stack := range cms.stacks {
// 		stores[key] = stack.CurrentStore()
// 		keys[key.Name()] = key
// 	}

// 	return cachemulti.NewFromKVStore(nil, stores, keys, nil, nil)
// }

func (cms *CacheMultiStack) Snapshot() int {
	var snapshot int
	for k := range cms.stacks {
		snapshot = cms.stacks[k].Snapshot()
	}
	return snapshot
}

func (cms *CacheMultiStack) RevertToSnapshot(target int) {
	for _, cacheStack := range cms.stacks {
		cacheStack.RevertToSnapshot(target)
	}
}

// GetStore returns an underlying Store by key.
func (cms CacheMultiStack) GetStore(key storetypes.StoreKey) storetypes.Store {
	stack := cms.stacks[key]
	if key == nil || stack == nil {
		panic(fmt.Sprintf("kv store with key %v has not been registered in stores", key))
	}
	return stack.CurrentStore()
}

// GetKVStore returns an underlying KVStore by key.
func (cms CacheMultiStack) GetKVStore(key storetypes.StoreKey) storetypes.KVStore {
	stack := cms.stacks[key]
	if key == nil || stack == nil {
		panic(fmt.Sprintf("kv store with key %v has not been registered in stores", key))
	}
	return stack.CurrentStore()
}
