//go:build test
// +build test

package mempool

// globalEVMMempool holds the global reference to the EVMMempool instance.
// It can only be set during application initialization.
var globalEVMMempool *EVMMempool

// SetGlobalEVMMempool sets the global EVMMempool instance.
// This should only be called during application initialization.
// In testing builds, it allows resetting by not returning an error.
func SetGlobalEVMMempool(mempool *EVMMempool) error {
	globalEVMMempool = mempool
	return nil
}

// GetGlobalEVMMempool returns the global EVMMempool instance.
// Returns nil if not set.
func GetGlobalEVMMempool() *EVMMempool {
	return globalEVMMempool
}

// ResetGlobalEVMMempool resets the global EVMMempool instance.
// This is intended for testing purposes only.
func ResetGlobalEVMMempool() {
	globalEVMMempool = nil
}
