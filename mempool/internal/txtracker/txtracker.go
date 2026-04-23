package txtracker

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/cosmos-sdk/telemetry"
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

// TxTracker tracks timestamps about important events in a transactions
// lifecycle and exposes metrics about these via prometheus.
type TxTracker struct {
	txCheckpoints map[common.Hash]*checkpoints
	lock          sync.RWMutex
}

// New creates a new Tracker instance.
func New() *TxTracker {
	return &TxTracker{
		txCheckpoints: make(map[common.Hash]*checkpoints),
	}
}

// Track initializes tracking for a tx. This should only be called from
// SendRawTransaction when a tx enters this node via a RPC.
func (t *TxTracker) Track(hash common.Hash) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if _, alreadyTracked := t.txCheckpoints[hash]; alreadyTracked {
		return fmt.Errorf("tx %s already being tracked", hash)
	}

	t.txCheckpoints[hash] = &checkpoints{TrackedAt: time.Now()}
	return nil
}

func (t *TxTracker) EnteredQueued(hash common.Hash) error {
	checkpoints, err := t.getCheckpointsIfTracked(hash)
	if err != nil {
		return fmt.Errorf("getting checkpoints for hash %s: %w", hash, err)
	}

	checkpoints.LastEnteredQueuedPoolAt = time.Now()
	telemetry.MeasureSince(checkpoints.TrackedAt, queuedInclusionLatencyKey) //nolint:staticcheck
	return nil
}

func (t *TxTracker) ExitedQueued(hash common.Hash) error {
	checkpoints, err := t.getCheckpointsIfTracked(hash)
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

func (t *TxTracker) EnteredPending(hash common.Hash) error {
	checkpoints, err := t.getCheckpointsIfTracked(hash)
	if err != nil {
		return fmt.Errorf("getting checkpoints for hash %s: %w", hash, err)
	}

	checkpoints.LastEnteredPendingPoolAt = time.Now()
	telemetry.MeasureSince(checkpoints.TrackedAt, pendingInclusionLatencyKey) //nolint:staticcheck
	return nil
}

func (t *TxTracker) ExitedPending(hash common.Hash) error {
	checkpoints, err := t.getCheckpointsIfTracked(hash)
	if err != nil {
		return fmt.Errorf("getting checkpoints for hash %s: %w", hash, err)
	}

	telemetry.MeasureSince(checkpoints.LastEnteredPendingPoolAt, pendingDurationKey) //nolint:staticcheck
	return nil
}

func (t *TxTracker) IncludedInBlock(hash common.Hash) error {
	checkpoints, err := t.getCheckpointsIfTracked(hash)
	if err != nil {
		return fmt.Errorf("getting checkpoints for hash %s: %w", hash, err)
	}

	telemetry.MeasureSince(checkpoints.TrackedAt, chainInclusionLatencyKey) //nolint:staticcheck
	return nil
}

func (t *TxTracker) RemovedFromPending(hash common.Hash) error {
	defer t.removeTx(hash)
	return t.ExitedPending(hash)
}

func (t *TxTracker) RemovedFromQueue(hash common.Hash) error {
	defer t.removeTx(hash)
	return t.ExitedQueued(hash)
}

func (t *TxTracker) getCheckpointsIfTracked(hash common.Hash) (*checkpoints, error) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	checkpoints, alreadyTracked := t.txCheckpoints[hash]
	if !alreadyTracked {
		return nil, fmt.Errorf("tx not already being tracked")
	}
	return checkpoints, nil
}

// removeTx removes a tx by hash.
func (t *TxTracker) removeTx(hash common.Hash) {
	t.lock.Lock()
	defer t.lock.Unlock()
	delete(t.txCheckpoints, hash)
}

// checkpoints is a set of important timestamps across a transactions lifecycle
// in the mempool.
type checkpoints struct {
	TrackedAt time.Time

	LastEnteredQueuedPoolAt time.Time

	LastEnteredPendingPoolAt time.Time
}
