package types

const (
	// ModuleName defines the module name
	ModuleName = "topholders"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_topholders"
)

// KVStore keys
var (
	// TopHoldersKey is the key for storing the cached top holders data
	TopHoldersKey = []byte{0x01}

	// LastUpdateKey is the key for storing the last update timestamp
	LastUpdateKey = []byte{0x02}
)
