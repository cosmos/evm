package mempool

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetGlobalEVMMempool(t *testing.T) {
	ResetGlobalEVMMempool()
	var wg sync.WaitGroup
	num := 10
	mempools := make([]*ExperimentalEVMMempool, num)
	for i := 0; i < num; i++ {
		mempools[i] = &ExperimentalEVMMempool{}
	}
	for i := 0; i < num; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			SetGlobalEVMMempool(mempools[idx])
		}(i)
	}
	wg.Wait()
	global := GetGlobalEVMMempool()
	require.NotNil(t, global)
	found := false
	for i := range num {
		if global == mempools[i] {
			found = true
			break
		}
	}
	require.True(t, found, "globalEVMMempool should be one of the provided mempools")
}
