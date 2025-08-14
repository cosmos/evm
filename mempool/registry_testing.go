//go:build test
// +build test

package mempool

// globalEVMMempool holds the global reference to the ExperimentalEVMMempool instance.
// It can only be set during application initialization.
var globalEVMMempool *ExperimentalEVMMempool

// SetGlobalEVMMempool sets the global ExperimentalEVMMempool instance.
// This should only be called during application initialization.
// In testing builds, it allows resetting by not returning an error.
func SetGlobalEVMMempool(mempool *ExperimentalEVMMempool) error {
	globalEVMMempool = mempool
	return nil
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
}
