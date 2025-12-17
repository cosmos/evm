// Copyright 2023 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package txpool

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/event"
	"github.com/holiman/uint256"
)

// LazyTransaction contains a small subset of the transaction properties that is
// enough for the miner and other APIs to handle large batches of transactions;
// and supports pulling up the entire transaction when really needed.
type LazyTransaction struct {
	Pool LazyResolver       // Transaction resolver to pull the real transaction up
	Hash common.Hash        // Transaction hash to pull up if needed
	Tx   *types.Transaction // Transaction if already resolved

	Time      time.Time    // Time when the transaction was first seen
	GasFeeCap *uint256.Int // Maximum fee per gas the transaction may consume
	GasTipCap *uint256.Int // Maximum miner tip per gas the transaction can pay

	Gas     uint64 // Amount of gas required by the transaction
	BlobGas uint64 // Amount of blob gas required by the transaction
}

// Resolve retrieves the full transaction belonging to a lazy handle if it is still
// maintained by the transaction pool.
//
// Note, the method will *not* cache the retrieved transaction if the original
// pool has not cached it. The idea being, that if the tx was too big to insert
// originally, silently saving it will cause more trouble down the line (and
// indeed seems to have caused a memory bloat in the original implementation
// which did just that).
func (ltx *LazyTransaction) Resolve() *types.Transaction {
	if ltx.Tx != nil {
		return ltx.Tx
	}
	return ltx.Pool.Get(ltx.Hash)
}

// LazyResolver is a minimal interface needed for a transaction pool to satisfy
// resolving lazy transactions. It's mostly a helper to avoid the entire sub-
// pool being injected into the lazy transaction.
type LazyResolver interface {
	// Get returns a transaction if it is contained in the pool, or nil otherwise.
	Get(hash common.Hash) *types.Transaction
}

// PendingFilter is a collection of filter rules to allow retrieving a subset
// of transactions for announcement or mining.
//
// Note, the entries here are not arbitrary useful filters, rather each one has
// a very specific call site in mind and each one can be evaluated very cheaply
// by the pool implementations. Only add new ones that satisfy those constraints.
type PendingFilter struct {
	MinTip  *uint256.Int // Minimum miner tip required to include a transaction
	BaseFee *uint256.Int // Minimum 1559 basefee needed to include a transaction
	BlobFee *uint256.Int // Minimum 4844 blobfee needed to include a blob transaction

	OnlyPlainTxs bool // Return only plain EVM transactions (peer-join announces, block space filling)
	OnlyBlobTxs  bool // Return only blob transactions (block blob-space filling)
}

// TxMetadata denotes the metadata of a transaction.
type TxMetadata struct {
	Type uint8  // The type of the transaction
	Size uint64 // The length of the 'rlp encoding' of a transaction
}

// RemovalReason is a string describing why a tx is being removed.
type RemovalReason string

// RemoveTxConfig configures how txs should be removed from the Subpool.
type RemoveTxConfig struct {
	// OutOfBound configures if the tx should be removed from the priced list
	// as well.
	OutOfBound bool

	// Unreserve configures if teh account will be relinquished to the main
	// txpool even if there are no references to it.
	Unreserve bool

	// StrictOverride determines if txs after the removed tx will also be
	// removed.
	StrictOverride *bool

	// Reason is the reason why this tx is being removed. Used for metrics.
	Reason RemovalReason
}

// NewRemoveTxConfig creates a new set of configuration options for removing
// txs.
func NewRemoveTxConfig() *RemoveTxConfig {
	return &RemoveTxConfig{}
}

// RemoveTxOption sets values on a RemoveTxConfig
type RemoveTxOption func(opts *RemoveTxConfig)

// WithOutOfBound configures if the tx should be removed from the priced list
// as well.
func WithOutOfBound() RemoveTxOption {
	return func(opts *RemoveTxConfig) {
		opts.OutOfBound = true
	}
}

// WithUnreserve is false, the account will not be relinquished to the main
// txpool even if there are no more references to it. This is used to handle a
// race when a tx being added, and it evicts a previously scheduled tx from the
// same account, which could lead to a premature release of the lock.
func WithUnreserve() RemoveTxOption {
	return func(opts *RemoveTxConfig) {
		opts.Unreserve = true
	}
}

// WithStrictOverride if not set will default to the lists default removing
// strictness. If set to true, this will force the list to remove all
// subsequent nonces tx after the tx being removed. If the tx is in pending and
// strict is true, it will enqueue all removed txs.
func WithStrictOverride(strict bool) RemoveTxOption {
	return func(opts *RemoveTxConfig) {
		opts.StrictOverride = &strict
	}
}

// WithRemovalReason specifies why a tx is being removed. This is for metrics.
func WithRemovalReason(reason RemovalReason) RemoveTxOption {
	return func(opts *RemoveTxConfig) {
		opts.Reason = reason
	}
}

// SubPool represents a specialized transaction pool that lives on its own (e.g.
// blob pool). Since independent of how many specialized pools we have, they do
// need to be updated in lockstep and assemble into one coherent view for block
// production, this interface defines the common methods that allow the primary
// transaction pool to manage the Subpools.
type SubPool interface {
	// Filter is a selector used to decide whether a transaction would be added
	// to this particular subpool.
	Filter(tx *types.Transaction) bool

	// Init sets the base parameters of the subpool, allowing it to load any saved
	// transactions from disk and also permitting internal maintenance routines to
	// start up.
	//
	// These should not be passed as a constructor argument - nor should the pools
	// start by themselves - in order to keep multiple Subpools in lockstep with
	// one another.
	Init(gasTip uint64, head *types.Header, reserver Reserver) error

	// Close terminates any background processing threads and releases any held
	// resources.
	Close() error

	// Reset retrieves the current state of the blockchain and ensures the content
	// of the transaction pool is valid with regard to the chain state.
	Reset(oldHead, newHead *types.Header)

	// SetGasTip updates the minimum price required by the subpool for a new
	// transaction, and drops all transactions below this threshold.
	SetGasTip(tip *big.Int)

	// Has returns an indicator whether subpool has a transaction cached with the
	// given hash.
	Has(hash common.Hash) bool

	// Get returns a transaction if it is contained in the pool, or nil otherwise.
	Get(hash common.Hash) *types.Transaction

	// GetRLP returns a RLP-encoded transaction if it is contained in the pool.
	GetRLP(hash common.Hash) []byte

	// GetMetadata returns the transaction type and transaction size with the
	// given transaction hash.
	GetMetadata(hash common.Hash) *TxMetadata

	// GetBlobs returns a number of blobs are proofs for the given versioned hashes.
	// This is a utility method for the engine API, enabling consensus clients to
	// retrieve blobs from the pools directly instead of the network.
	GetBlobs(vhashes []common.Hash) ([]*kzg4844.Blob, []*kzg4844.Proof)

	// ValidateTxBasics checks whether a transaction is valid according to the consensus
	// rules, but does not check state-dependent validation such as sufficient balance.
	// This check is meant as a static check which can be performed without holding the
	// pool mutex.
	ValidateTxBasics(tx *types.Transaction) error

	// Add enqueues a batch of transactions into the pool if they are valid. Due
	// to the large transaction churn, add may postpone fully integrating the tx
	// to a later point to batch multiple ones together.
	Add(txs []*types.Transaction, sync bool) []error

	// Pending retrieves all currently processable transactions, grouped by origin
	// account and sorted by nonce.
	//
	// The transactions can also be pre-filtered by the dynamic fee components to
	// reduce allocations and load on downstream subsystems.
	Pending(filter PendingFilter) map[common.Address][]*LazyTransaction

	// SubscribeTransactions subscribes to new transaction events. The subscriber
	// can decide whether to receive notifications only for newly seen transactions
	// or also for reorged out ones.
	SubscribeTransactions(ch chan<- core.NewTxsEvent, reorgs bool) event.Subscription

	// Nonce returns the next nonce of an account, with all transactions executable
	// by the pool already applied on top.
	Nonce(addr common.Address) uint64

	// Stats retrieves the current pool stats, namely the number of pending and the
	// number of queued (non-executable) transactions.
	Stats() (int, int)

	// Content retrieves the data content of the transaction pool, returning all the
	// pending as well as queued transactions, grouped by account and sorted by nonce.
	Content() (map[common.Address][]*types.Transaction, map[common.Address][]*types.Transaction)

	// ContentFrom retrieves the data content of the transaction pool, returning the
	// pending as well as queued transactions of this address, grouped by nonce.
	ContentFrom(addr common.Address) ([]*types.Transaction, []*types.Transaction)

	// Status returns the known status (unknown/pending/queued) of a transaction
	// identified by their hashes.
	Status(hash common.Hash) TxStatus

	// Clear removes all tracked transactions from the pool
	Clear()

	// RemoveTx removes a tracked transaction from the pool
	RemoveTx(hash common.Hash, opts ...RemoveTxOption) int
}
