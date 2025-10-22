package types

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
)

const (
	// ModuleName string name of module
	ModuleName = "evm"

	// StoreKey key for ethereum storage data, account code (StateDB) or block
	// related data for Web3.
	// The EVM module should use a prefix store.
	StoreKey = ModuleName

	// ObjectKey is the key to access the EVM object store, that is reset
	// during the Commit phase.
	ObjectKey = "object:" + ModuleName

	// RouterKey uses module name for routing
	RouterKey = ModuleName
)

// prefix bytes for the EVM persistent store
const (
	prefixCode = iota + 1
	prefixStorage
	prefixParams
	prefixCodeHash
	prefixEvmCoinInfo
)

// prefix bytes for the EVM object store
const (
	prefixObjectBloom = iota + 1
	prefixObjectGasUsed
)

// KVStore key prefixes
var (
	KeyPrefixCode        = []byte{prefixCode}
	KeyPrefixStorage     = []byte{prefixStorage}
	KeyPrefixParams      = []byte{prefixParams}
	KeyPrefixCodeHash    = []byte{prefixCodeHash}
	KeyPrefixEvmCoinInfo = []byte{prefixEvmCoinInfo}
)

// Object Store key prefixes
var (
	KeyPrefixObjectBloom   = []byte{prefixObjectBloom}
	KeyPrefixObjectGasUsed = []byte{prefixObjectGasUsed}
)

// AddressStoragePrefix returns a prefix to iterate over a given account storage.
func AddressStoragePrefix(address common.Address) []byte {
	return append(KeyPrefixStorage, address.Bytes()...)
}

// StateKey defines the full key under which an account state is stored.
func StateKey(address common.Address, key []byte) []byte {
	return append(AddressStoragePrefix(address), key...)
}

func ObjectGasUsedKey(txIndex int) []byte {
	var key [1 + 8]byte
	key[0] = prefixObjectGasUsed
	binary.BigEndian.PutUint64(key[1:], uint64(txIndex)) //nolint:gosec
	return key[:]
}

func ObjectBloomKey(txIndex, msgIndex int) []byte {
	var key [1 + 8 + 8]byte
	key[0] = prefixObjectBloom
	binary.BigEndian.PutUint64(key[1:], uint64(txIndex))  //nolint:gosec
	binary.BigEndian.PutUint64(key[9:], uint64(msgIndex)) //nolint:gosec
	return key[:]
}
