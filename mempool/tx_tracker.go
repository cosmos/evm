package mempool

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	// import for init side effects
	_ "github.com/cosmos/cosmos-sdk/telemetry"

	"github.com/cosmos/evm/mempool/txpool/legacypool"
)

var (
	meter = otel.Meter("cosmos-evm/mempool")

	// chainInclusionLatency measures how long it takes for a transaction to go
	// from initially being tracked to being included on chain
	chainInclusionLatency metric.Int64Histogram

	// queuedInclusionLatency measures how long it takes for a transaction to go
	// from initially being tracked to being included in queued
	queuedInclusionLatency metric.Int64Histogram

	// pendingInclusionLatency measures how long it takes for a transaction to
	// go from initially being tracked to being included in pending
	pendingInclusionLatency metric.Int64Histogram

	// queuedDuration is how long a transaction is in the queued pool for
	// before exiting. Only recorded on exit (if a tx stays in the pool
	// forever, this will not be recorded).
	queuedDuration metric.Int64Histogram

	// pendingDuration is how long a transaction is in the pending pool for
	// before exiting. Only recorded on exit (if a tx stays in the pool
	// forever, this will not be recorded).
	pendingDuration metric.Int64Histogram

	// bounds is the bucket intervals to break histograms into.
	bounds = []float64{50, 100, 250, 500, 1000, 1500, 2000, 5000, 10000, 15000, 20000, 30000, 60000, 90000, 120000, 300000}

	// unit is the unit identifier histograms are recorded in.
	unit = "ms"
)

func init() {
	var err error
	chainInclusionLatency, err = meter.Int64Histogram("mempool.chain_inclusion_latency", metric.WithUnit(unit), metric.WithExplicitBucketBoundaries(bounds...))
	if err != nil {
		panic(err)
	}
	queuedInclusionLatency, err = meter.Int64Histogram("mempool.queued_inclusion_latency", metric.WithUnit(unit), metric.WithExplicitBucketBoundaries(bounds...))
	if err != nil {
		panic(err)
	}
	pendingInclusionLatency, err = meter.Int64Histogram("mempool.pending_inclusion_latency", metric.WithUnit(unit), metric.WithExplicitBucketBoundaries(bounds...))
	if err != nil {
		panic(err)
	}
	queuedDuration, err = meter.Int64Histogram("mempool.queued_duration", metric.WithUnit(unit), metric.WithExplicitBucketBoundaries(bounds...))
	if err != nil {
		panic(err)
	}
	pendingDuration, err = meter.Int64Histogram("mempool.pending_duration", metric.WithUnit(unit), metric.WithExplicitBucketBoundaries(bounds...))
	if err != nil {
		panic(err)
	}
}

// txTracker tracks timestamps about important events in a transactions
// lifecycle and exposes metrics about these via prometheus.
type txTracker struct {
	txCheckpoints map[common.Hash]*checkpoints
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
	if _, alreadyTrakced := txt.txCheckpoints[hash]; alreadyTrakced {
		return fmt.Errorf("tx %s already being tracked", hash)
	}

	txt.txCheckpoints[hash] = &checkpoints{TrackedAt: time.Now()}
	return nil
}

func (txt *txTracker) EnteredQueued(hash common.Hash) error {
	checkpoints, alreadyTrakced := txt.txCheckpoints[hash]
	if !alreadyTrakced {
		return fmt.Errorf("tx %s not already being tracked", hash)
	}

	checkpoints.LastEnteredQueuedPoolAt = time.Now()
	queuedInclusionLatency.Record(context.Background(), checkpoints.QueuedInclusionLatency().Milliseconds())
	return nil
}

func (txt *txTracker) ExitedQueued(hash common.Hash) error {
	checkpoints, alreadyTrakced := txt.txCheckpoints[hash]
	if !alreadyTrakced {
		return fmt.Errorf("tx %s not already being tracked", hash)
	}

	checkpoints.LastExitedQueuedPoolAt = time.Now()
	queuedDuration.Record(context.Background(), checkpoints.TimeInQueuedPool().Milliseconds())
	return nil
}

func (txt *txTracker) EnteredPending(hash common.Hash) error {
	checkpoints, alreadyTrakced := txt.txCheckpoints[hash]
	if !alreadyTrakced {
		return fmt.Errorf("tx %s not already being tracked", hash)
	}

	checkpoints.LastEnteredPendingPoolAt = time.Now()
	pendingInclusionLatency.Record(context.Background(), checkpoints.PendingInclusionLatency().Milliseconds())
	return nil
}

func (txt *txTracker) ExitedPending(hash common.Hash) error {
	checkpoints, alreadyTrakced := txt.txCheckpoints[hash]
	if !alreadyTrakced {
		return fmt.Errorf("tx %s not already being tracked", hash)
	}

	checkpoints.LastExitedPendingPoolAt = time.Now()
	pendingDuration.Record(context.Background(), checkpoints.TimeInPendingPool().Milliseconds())
	return nil
}

func (txt *txTracker) IncludedInBlock(hash common.Hash) error {
	checkpoints, alreadyTrakced := txt.txCheckpoints[hash]
	if !alreadyTrakced {
		return fmt.Errorf("tx %s not already being tracked", hash)
	}

	checkpoints.IncludedInBlockAt = time.Now()
	chainInclusionLatency.Record(context.Background(), checkpoints.InclusionLatency().Milliseconds())
	return nil
}

// RemoveTx tracks final values for a tx as it exists the mempool and removes
// it from the txTracker.
func (txt *txTracker) RemoveTx(hash common.Hash, pool legacypool.PoolType) error {
	defer delete(txt.txCheckpoints, hash)

	switch pool {
	case legacypool.Pending:
		return txt.ExitedPending(hash)
	case legacypool.Queue:
		return txt.ExitedQueued(hash)
	}

	return nil
}

// checkpoints is a set of important timestamps across a transactions lifecycle
// in the mempool.
type checkpoints struct {
	TrackedAt time.Time

	LastEnteredQueuedPoolAt time.Time
	LastExitedQueuedPoolAt  time.Time

	LastEnteredPendingPoolAt time.Time
	LastExitedPendingPoolAt  time.Time

	IncludedInBlockAt time.Time
}

func (c *checkpoints) TimeInQueuedPool() time.Duration {
	if c.LastEnteredQueuedPoolAt.IsZero() {
		// It is possible that a tx never entered the queued pool when we call
		// this (directly replaced a tx in the pending pool), thus we simply
		// return 0 if the tx never entered the queued pool.
		return time.Duration(0)
	}
	return c.LastExitedQueuedPoolAt.Sub(c.LastEnteredQueuedPoolAt)
}

func (c *checkpoints) TimeInPendingPool() time.Duration {
	return c.LastExitedPendingPoolAt.Sub(c.LastEnteredPendingPoolAt)
}

func (c *checkpoints) InclusionLatency() time.Duration {
	return c.IncludedInBlockAt.Sub(c.TrackedAt)
}

func (c *checkpoints) QueuedInclusionLatency() time.Duration {
	return c.LastEnteredQueuedPoolAt.Sub(c.TrackedAt)
}

func (c *checkpoints) PendingInclusionLatency() time.Duration {
	return c.LastEnteredPendingPoolAt.Sub(c.TrackedAt)
}
