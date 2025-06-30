package snapshotmulti

import (
	"crypto/rand"
	"flag"
	"fmt"
	"testing"

	dbm "github.com/cosmos/cosmos-db"

	"cosmossdk.io/store/cachekv"
	"cosmossdk.io/store/dbadapter"
	"cosmossdk.io/store/types"
)

var (
	benchSink    interface{}
	benchTxCount = flag.Int("bench.numtx", 256, "number of transactions per benchmark run")
)

// genBytes returns a byte slice of the given length filled with random bytes.
func genBytes(n int) []byte {
	bz := make([]byte, n)
	if _, err := rand.Read(bz); err != nil {
		panic(err)
	}
	return bz
}

func setupCacheMultiStoreWithKeys(numStores, numEntries int) (*Store, []*types.KVStoreKey) {
	storeMap := make(map[*types.KVStoreKey]types.CacheWrap, numStores)
	keys := make([]*types.KVStoreKey, numStores)
	for i := 0; i < numStores; i++ {
		key := types.NewKVStoreKey(fmt.Sprintf("store%d", i))
		kv := cachekv.NewStore(dbadapter.Store{DB: dbm.NewMemDB()})
		for j := 0; j < numEntries; j++ {
			kv.Set([]byte(fmt.Sprintf("%s-key-%d", key.Name(), j)), genBytes(32))
		}
		storeMap[key] = kv
		keys[i] = key
	}
	return NewStoreWithKVStores(storeMap), keys
}

func benchmarkSequential(b *testing.B) {
	b.Helper()
	cms, keys := setupCacheMultiStoreWithKeys(20, 200000)
	selected := keys[:5]
	txs := *benchTxCount

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for tx := 0; tx < txs; tx++ {
			for j := 0; j < 10; j++ {
				for _, key := range selected {
					kv := cms.GetKVStore(key)
					kv.Get([]byte(fmt.Sprintf("%s-key-%d", key.Name(), j%20)))
					kv.Set([]byte(fmt.Sprintf("%s-tx-%d-%d", key.Name(), tx, j)), genBytes(32))
				}
				snapshot := cms.Snapshot()
				benchSink = snapshot
			}
		}
	}
}

func BenchmarkSequentialCacheMultiStore(b *testing.B) {
	benchmarkSequential(b)
}
