package txtracker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("github.com/cosmos/evm/mempool/internal/txtracker")

var (
	// chainInclusionLatency measures how long it takes for a transaction to go
	// from initially being tracked to being included on chain.
	chainInclusionLatency metric.Float64Histogram

	// queuedInclusionLatency measures how long it takes for a transaction to
	// go from initially being tracked to being included in queued.
	queuedInclusionLatency metric.Float64Histogram

	// pendingInclusionLatency measures how long it takes for a transaction to
	// go from initially being tracked to being included in pending.
	pendingInclusionLatency metric.Float64Histogram

	// queuedDuration is how long a transaction is in the queued pool for
	// before exiting. Only recorded on exit (if a tx stays in the pool
	// forever, this will not be recorded).
	queuedDuration metric.Float64Histogram

	// pendingDuration is how long a transaction is in the pending pool for
	// before exiting. Only recorded on exit (if a tx stays in the pool
	// forever, this will not be recorded).
	pendingDuration metric.Float64Histogram
)

func init() {
	timer := func(name, desc string) metric.Float64Histogram {
		m, err := meter.Float64Histogram(name, metric.WithDescription(desc), metric.WithUnit("ms"))
		if err != nil {
			panic(err)
		}
		return m
	}

	chainInclusionLatency = timer("txpool.tracker.chain_inclusion_latency", "Time from initial tracking to inclusion on chain")
	queuedInclusionLatency = timer("txpool.tracker.queued_inclusion_latency", "Time from initial tracking to entering the queued pool")
	pendingInclusionLatency = timer("txpool.tracker.pending_inclusion_latency", "Time from initial tracking to entering the pending pool")
	queuedDuration = timer("txpool.tracker.queued_duration", "Time spent in the queued pool before exit")
	pendingDuration = timer("txpool.tracker.pending_duration", "Time spent in the pending pool before exit")
}

func recordSince(h metric.Float64Histogram, start time.Time) {
	h.Record(context.Background(), float64(time.Since(start).Milliseconds()))
}

// TxTracker tracks timestamps about important events in a transactions
// lifecycle and exposes metrics about these via otel.
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
	recordSince(queuedInclusionLatency, checkpoints.TrackedAt)
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
	recordSince(queuedDuration, checkpoints.LastEnteredQueuedPoolAt)
	return nil
}

func (t *TxTracker) EnteredPending(hash common.Hash) error {
	checkpoints, err := t.getCheckpointsIfTracked(hash)
	if err != nil {
		return fmt.Errorf("getting checkpoints for hash %s: %w", hash, err)
	}

	checkpoints.LastEnteredPendingPoolAt = time.Now()
	recordSince(pendingInclusionLatency, checkpoints.TrackedAt)
	return nil
}

func (t *TxTracker) ExitedPending(hash common.Hash) error {
	checkpoints, err := t.getCheckpointsIfTracked(hash)
	if err != nil {
		return fmt.Errorf("getting checkpoints for hash %s: %w", hash, err)
	}

	recordSince(pendingDuration, checkpoints.LastEnteredPendingPoolAt)
	return nil
}

func (t *TxTracker) IncludedInBlock(hash common.Hash) error {
	checkpoints, err := t.getCheckpointsIfTracked(hash)
	if err != nil {
		return fmt.Errorf("getting checkpoints for hash %s: %w", hash, err)
	}

	recordSince(chainInclusionLatency, checkpoints.TrackedAt)
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
