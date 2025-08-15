package types

import (
	"reflect"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// RpcBlock struct is defined to include all fields from types.Block, including private ones.
// This allows us to access and print all fields, including those that are not exported (private).
type RpcBlock struct {
	Header       *types.Header
	Uncles       []*types.Header
	Transactions []*types.Transaction
	Withdrawals  []*types.Withdrawal

	// Cache fields
	Hash *atomic.Pointer[common.Hash] `json:"hash"`
	Size *atomic.Uint64               `json:"size"`

	// Metadata fields
	ReceivedAt   time.Time   `json:"received_at"`
	ReceivedFrom interface{} `json:"received_from"`
}

// NewRpcBlock creates a new RpcBlock from a ethereum Block.
func NewRPCBlock(block *types.Block) *RpcBlock {
	// Getting private fields via reflection
	blockValue := reflect.ValueOf(block).Elem()

	// Accessing private fields: hash and size
	hashField := blockValue.FieldByName("hash")
	hashPtr := (*atomic.Pointer[common.Hash])(unsafe.Pointer(hashField.UnsafeAddr()))

	sizeField := blockValue.FieldByName("size")
	sizePtr := (*atomic.Uint64)(unsafe.Pointer(sizeField.UnsafeAddr()))
	return &RpcBlock{
		Header:       block.Header(),
		Uncles:       block.Uncles(),
		Transactions: block.Transactions(),
		Withdrawals:  block.Withdrawals(),
		Hash:         hashPtr, // Assign pointer directly to avoid copying lock value
		Size:         sizePtr, // Assign pointer directly to avoid copying lock value
		ReceivedAt:   blockValue.FieldByName("ReceivedAt").Interface().(time.Time),
		ReceivedFrom: blockValue.FieldByName("ReceivedFrom").Interface(),
	}
}
