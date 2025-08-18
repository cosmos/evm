package mempool

import (
	"sync"
)

// globalEVMMempool holds the global reference to the ExperimentalEVMMempool instance.
// It can only be set during application initialization.
var (
	globalEVMMempool     *ExperimentalEVMMempool
	globalEVMMempoolSync sync.Once
)

// SetGlobalEVMMempool sets the global ExperimentalEVMMempool instance.
// This is guaranteed to only set the instance once for the lifetime of the process,
// even if called multiple times. Should only be called during application initialization.
func SetGlobalEVMMempool(mempool *ExperimentalEVMMempool) {
	globalEVMMempoolSync.Do(func() {
		globalEVMMempool = mempool
	})
}

// GetGlobalEVMMempool returns the global ExperimentalEVMMempool instance.
// Returns nil if not set.
func GetGlobalEVMMempool() *ExperimentalEVMMempool {
	return globalEVMMempool
}

// ResetGlobalEVMMempool resets the global ExperimentalEVMMempool instance.
// This is intended for testing purposes only.
func ResetGlobalEVMMempool() {
	globalEVMMempool = nil
	globalEVMMempoolSync = sync.Once{}
}
