//go:build !test
// +build !test

package mempool

import "errors"

// globalEVMMempool holds the global reference to the EVMMempool instance.
// It can only be set during application initialization.
var globalEVMMempool *EVMMempool

// SetGlobalEVMMempool sets the global EVMMempool instance.
// This should only be called during application initialization.
// In production builds, it returns an error if already set.
func SetGlobalEVMMempool(mempool *EVMMempool) error {
	if globalEVMMempool != nil {
		return errors.New("global EVM mempool already set")
	}
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
