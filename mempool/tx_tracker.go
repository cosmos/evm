package mempool

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/cosmos-sdk/telemetry"

	"github.com/cosmos/evm/mempool/txpool/legacypool"
)

var (
	// chainInclusionLatencyKey measures how long it takes for a transaction to go
	// from initially being tracked to being included on chain
	chainInclusionLatencyKey = "chain_inclusion_latency"

	// queuedInclusionLatencyKey measures how long it takes for a transaction to go
	// from initially being tracked to being included in queued
	queuedInclusionLatencyKey = "queued_inclusion_latency"

	// pendingInclusionLatencyKey measures how long it takes for a transaction to
	// go from initially being tracked to being included in pending
	pendingInclusionLatencyKey = "pending_inclusion_latency"

	// queuedDuration is how long a transaction is in the queued pool for
	// before exiting. Only recorded on exit (if a tx stays in the pool
	// forever, this will not be recorded).
	queuedDurationKey = "queued_duration"

	// pendingDuration is how long a transaction is in the pending pool for
	// before exiting. Only recorded on exit (if a tx stays in the pool
	// forever, this will not be recorded).
	pendingDurationKey = "pending_duration"
)

// txTracker tracks timestamps about important events in a transactions
// lifecycle and exposes metrics about these via prometheus.
type txTracker struct {
	txCheckpoints map[common.Hash]*checkpoints
	lock          sync.RWMutex
}

// newTxTracker creates a new txTracker instance
func newTxTracker() *txTracker {
	return &txTracker{
		txCheckpoints: make(map[common.Hash]*checkpoints),
	}
}

// Track initializes tracking for a tx. This should only be called from
// SendRawTransaction when a tx enters this node via a RPC.
func (txt *txTracker) Track(hash common.Hash) error {
	txt.lock.Lock()
	defer txt.lock.Unlock()

	if _, alreadyTracked := txt.txCheckpoints[hash]; alreadyTracked {
		return fmt.Errorf("tx %s already being tracked", hash)
	}

	txt.txCheckpoints[hash] = &checkpoints{TrackedAt: time.Now()}
	return nil
}

func (txt *txTracker) EnteredQueued(hash common.Hash) error {
	checkpoints, err := txt.getCheckpointsIfTracked(hash)
	if err != nil {
		return fmt.Errorf("getting checkpoints for hash %s: %w", hash, err)
	}

	checkpoints.LastEnteredQueuedPoolAt = time.Now()
	telemetry.MeasureSince(checkpoints.TrackedAt, queuedInclusionLatencyKey) //nolint:staticcheck
	return nil
}

func (txt *txTracker) ExitedQueued(hash common.Hash) error {
	checkpoints, err := txt.getCheckpointsIfTracked(hash)
	if err != nil {
		return fmt.Errorf("getting checkpoints for hash %s: %w", hash, err)
	}

	if checkpoints.LastEnteredQueuedPoolAt.IsZero() {
		// It is possible that a tx never entered the queued pool when we call
		// this (directly replaced a tx in the pending pool). In this case we
		// dont record the duration
		return nil
	}
	telemetry.MeasureSince(checkpoints.LastEnteredQueuedPoolAt, queuedDurationKey) //nolint:staticcheck
	return nil
}

func (txt *txTracker) EnteredPending(hash common.Hash) error {
	checkpoints, err := txt.getCheckpointsIfTracked(hash)
	if err != nil {
		return fmt.Errorf("getting checkpoints for hash %s: %w", hash, err)
	}

	checkpoints.LastEnteredPendingPoolAt = time.Now()
	telemetry.MeasureSince(checkpoints.TrackedAt, pendingInclusionLatencyKey) //nolint:staticcheck
	return nil
}

func (txt *txTracker) ExitedPending(hash common.Hash) error {
	checkpoints, err := txt.getCheckpointsIfTracked(hash)
	if err != nil {
		return fmt.Errorf("getting checkpoints for hash %s: %w", hash, err)
	}

	telemetry.MeasureSince(checkpoints.LastEnteredPendingPoolAt, pendingDurationKey) //nolint:staticcheck
	return nil
}

func (txt *txTracker) IncludedInBlock(hash common.Hash) error {
	checkpoints, err := txt.getCheckpointsIfTracked(hash)
	if err != nil {
		return fmt.Errorf("getting checkpoints for hash %s: %w", hash, err)
	}

	telemetry.MeasureSince(checkpoints.TrackedAt, chainInclusionLatencyKey) //nolint:staticcheck
	return nil
}

func (txt *txTracker) getCheckpointsIfTracked(hash common.Hash) (*checkpoints, error) {
	txt.lock.RLock()
	defer txt.lock.RUnlock()

	checkpoints, alreadyTracked := txt.txCheckpoints[hash]
	if !alreadyTracked {
		return nil, fmt.Errorf("tx not already being tracked")
	}
	return checkpoints, nil
}

// RemoveTx tracks final values for a tx as it exists the mempool and removes
// it from the txTracker.
func (txt *txTracker) RemoveTx(hash common.Hash, pool legacypool.PoolType) error {
	defer txt.removeTx(hash)

	switch pool {
	case legacypool.Pending:
		return txt.ExitedPending(hash)
	case legacypool.Queue:
		return txt.ExitedQueued(hash)
	}

	return nil
}

// removeTx removes a tx by hash.
func (txt *txTracker) removeTx(hash common.Hash) {
	txt.lock.Lock()
	defer txt.lock.Unlock()
	delete(txt.txCheckpoints, hash)
}

// checkpoints is a set of important timestamps across a transactions lifecycle
// in the mempool.
type checkpoints struct {
	TrackedAt time.Time

	LastEnteredQueuedPoolAt time.Time

	LastEnteredPendingPoolAt time.Time
}
