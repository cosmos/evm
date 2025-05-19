package statedb

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// transientStorage is a representation of EIP-1153 "Transient Storage".
type transientStorage map[common.Address]Storage

// newTransientStorage creates a new instance of a transientStorage.
func newTransientStorage() transientStorage {
	return make(transientStorage)
}

// Set sets the transient-storage `value` for `key` at the given `addr`.
func (t transientStorage) Set(addr common.Address, key, value common.Hash) {
	if value == (common.Hash{}) { // this is a 'delete'
		if _, ok := t[addr]; ok {
			delete(t[addr], key)
			if len(t[addr]) == 0 {
				delete(t, addr)
			}
		}
	} else {
		if _, ok := t[addr]; !ok {
			t[addr] = make(Storage)
		}
		t[addr][key] = value
	}
}

// Get gets the transient storage for `key` at the given `addr`.
func (t transientStorage) Get(addr common.Address, key common.Hash) common.Hash {
	val, ok := t[addr]
	if !ok {
		return common.Hash{}
	}
	return val[key]
}

// Copy does a deep copy of the transientStorage
func (t transientStorage) Copy() transientStorage {
	storage := make(transientStorage)
	for key, value := range t {
		storage[key] = value.Copy()
	}
	return storage
}

// PrettyPrint prints the contents of the access list in a human-readable form
func (t transientStorage) PrettyPrint() string {
	out := new(strings.Builder)
	var sortedAddrs []common.Address
	for addr := range t {
		sortedAddrs = append(sortedAddrs, addr)
		slices.SortFunc(sortedAddrs, common.Address.Cmp)
	}

	for _, addr := range sortedAddrs {
		fmt.Fprintf(out, "%#x:", addr)
		var sortedKeys []common.Hash
		storage := t[addr]
		for key := range storage {
			sortedKeys = append(sortedKeys, key)
		}
		slices.SortFunc(sortedKeys, common.Hash.Cmp)
		for _, key := range sortedKeys {
			fmt.Fprintf(out, "  %X : %X\n", key, storage[key])
		}
	}
	return out.String()
}
