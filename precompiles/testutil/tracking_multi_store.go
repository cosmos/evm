package testutil

import (
	"github.com/stretchr/testify/require"
	"io"
	"testing"
	"time"

	storetypes "cosmossdk.io/store/types"
)

// TrackingMultiStore implements the CacheMultiStore interface, but tracks calls to the Write interface as
// well ass the branches created via CacheMultiStore()
type TrackingMultiStore struct {
	Store            storetypes.CacheMultiStore
	Writes           int
	WriteTS          *time.Time
	HistoricalStores []*TrackingMultiStore
}

func (t *TrackingMultiStore) GetStoreType() storetypes.StoreType {
	return t.Store.GetStoreType()
}

func (t *TrackingMultiStore) CacheWrap() storetypes.CacheWrap {
	return t.Store.CacheWrap()
}

func (t *TrackingMultiStore) CacheWrapWithTrace(w io.Writer, tc storetypes.TraceContext) storetypes.CacheWrap {
	return t.Store.CacheWrapWithTrace(w, tc)
}

func (t *TrackingMultiStore) CacheMultiStoreWithVersion(version int64) (storetypes.CacheMultiStore, error) {
	return t.CacheMultiStoreWithVersion(version)
}

func (t *TrackingMultiStore) GetStore(key storetypes.StoreKey) storetypes.Store {
	return t.Store.GetStore(key)
}

func (t *TrackingMultiStore) GetKVStore(key storetypes.StoreKey) storetypes.KVStore {
	return t.Store.GetKVStore(key)
}

func (t *TrackingMultiStore) TracingEnabled() bool {
	return t.Store.TracingEnabled()
}

func (t *TrackingMultiStore) SetTracer(w io.Writer) storetypes.MultiStore {
	return t.Store.SetTracer(w)
}

func (t *TrackingMultiStore) SetTracingContext(context storetypes.TraceContext) storetypes.MultiStore {
	return t.Store.SetTracingContext(context)
}

func (t *TrackingMultiStore) LatestVersion() int64 {
	return t.Store.LatestVersion()
}

func (t *TrackingMultiStore) Copy() storetypes.CacheMultiStore {
	return t.Store.Copy()
}

func (t *TrackingMultiStore) Write() {
	t.Writes++
	now := time.Now()
	t.WriteTS = &now
	t.Store.Write()
}

func (t *TrackingMultiStore) CacheMultiStore() storetypes.CacheMultiStore {
	cms := t.Store.CacheMultiStore()
	tms := &TrackingMultiStore{Store: cms}
	t.HistoricalStores = append(t.HistoricalStores, tms)
	return tms
}

// ValidateWrites tests the number of writes to a tree of tracking multi stores,
// and that all the writes in a branching cache multistore/cascade upwards
func ValidateWrites(t *testing.T, ms *TrackingMultiStore, expWrites int) {
	toTestCMS := []*TrackingMultiStore{ms}
	writes := 0
	var writeTS *time.Time
	for len(toTestCMS) > 0 {
		currCMS := toTestCMS[0]
		toTestCMS = toTestCMS[1:]
		writes += currCMS.Writes
		if currCMS.WriteTS != nil {
			if writeTS != nil {
				// assert that branches with higher depth were written first
				require.True(t, currCMS.WriteTS.Before(*writeTS))
			}
			writeTS = currCMS.WriteTS
		}
		if len(currCMS.HistoricalStores) > 0 {
			for _, s := range currCMS.HistoricalStores {
				toTestCMS = append(toTestCMS, s)
			}
		}
	}
	require.Equal(t, expWrites, writes)
}
