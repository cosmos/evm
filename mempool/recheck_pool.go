package mempool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"cosmossdk.io/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
)

var (
	meter           = otel.Meter("github.com/cosmos/evm/mempool")
	recheckDuration metric.Float64Histogram
)

func init() {
	var err error
	recheckDuration, err = meter.Float64Histogram(
		"mempool.recheck.duration",
		metric.WithDescription("Duration of cosmos mempool recheck loop"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		panic(err)
	}
}

// RecheckMempool wraps an ExtMempool and provides event-driven rechecking
// of transactions when new blocks are committed. It mirrors the legacypool
// pattern but simplified for Cosmos semantics (no reorgs, no nonce tracking).
//
// All pool mutations (Insert, Remove) and reads (Select, CountTx) are protected
// by a RWMutex to ensure thread-safety during recheck operations.
type RecheckMempool struct {
	sdkmempool.ExtMempool

	// mu protects the pool during mutations and reads.
	// Write lock: Insert, Remove, runRecheck
	// Read lock: Select, CountTx
	mu sync.RWMutex

	anteHandler sdk.AnteHandler
	getCtx      func() (sdk.Context, error)
	logger      log.Logger

	// Event channels
	reqRecheckCh    chan struct{}
	recheckDoneCh   chan chan struct{}
	shutdownCh      chan struct{}
	shutdownOnce    sync.Once
	recheckShutdown chan struct{} // closed when scheduleRecheckLoop exits

	wg sync.WaitGroup
}

// NewRecheckMempool creates a new RecheckMempool wrapping the given pool.
func NewRecheckMempool(
	logger log.Logger,
	pool sdkmempool.ExtMempool,
	anteHandler sdk.AnteHandler,
	getCtx func() (sdk.Context, error),
) *RecheckMempool {
	return &RecheckMempool{
		ExtMempool:      pool,
		anteHandler:     anteHandler,
		getCtx:          getCtx,
		logger:          logger.With(log.ModuleKey, "RecheckMempool"),
		reqRecheckCh:    make(chan struct{}),
		recheckDoneCh:   make(chan chan struct{}),
		shutdownCh:      make(chan struct{}),
		recheckShutdown: make(chan struct{}),
	}
}

// Start begins the background recheck loop.
func (m *RecheckMempool) Start() {
	m.wg.Add(1)
	go m.scheduleRecheckLoop()
}

// Close gracefully shuts down the recheck loop.
func (m *RecheckMempool) Close() error {
	m.shutdownOnce.Do(func() {
		close(m.shutdownCh)
	})
	m.wg.Wait()
	return nil
}

// Insert adds a transaction to the pool after running the ante handler.
// This is the main entry point for new cosmos transactions.
func (m *RecheckMempool) Insert(goCtx context.Context, tx sdk.Tx) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx := sdk.UnwrapSDKContext(goCtx)

	// Branch the multistore so ante handler writes don't affect state on failure
	ms := ctx.MultiStore()
	msCache := ms.CacheMultiStore()
	ctx = ctx.WithMultiStore(msCache)

	// Run ante handler for validation
	if _, err := m.anteHandler(ctx, tx, false); err != nil {
		return fmt.Errorf("ante handler failed: %w", err)
	}

	// Ante handler succeeded - commit state changes
	msCache.Write()

	// Insert into underlying pool
	return m.ExtMempool.Insert(goCtx, tx)
}

// Remove removes a transaction from the pool.
func (m *RecheckMempool) Remove(tx sdk.Tx) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ExtMempool.Remove(tx)
}

// Select returns an iterator over transactions in the pool.
func (m *RecheckMempool) Select(ctx context.Context, txs [][]byte) sdkmempool.Iterator {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ExtMempool.Select(ctx, txs)
}

// CountTx returns the number of transactions in the pool.
func (m *RecheckMempool) CountTx() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ExtMempool.CountTx()
}

// TriggerRecheck signals that a new block arrived and returns a channel
// that closes when the recheck completes (or is superseded by another).
func (m *RecheckMempool) TriggerRecheck() <-chan struct{} {
	select {
	case m.reqRecheckCh <- struct{}{}:
		return <-m.recheckDoneCh
	case <-m.recheckShutdown:
		ch := make(chan struct{})
		close(ch)
		return ch
	}
}

// TriggerRecheckSync triggers a recheck and blocks until complete.
func (m *RecheckMempool) TriggerRecheckSync() {
	<-m.TriggerRecheck()
}

// scheduleRecheckLoop is the main event loop that coordinates recheck execution.
// Only one recheck runs at a time. If a new block arrives while a recheck is
// running, the current recheck is cancelled and a new one is scheduled.
func (m *RecheckMempool) scheduleRecheckLoop() {
	defer m.wg.Done()
	defer close(m.recheckShutdown)

	var (
		curDone       chan struct{} // non-nil while recheck is running
		nextDone      = make(chan struct{})
		launchNextRun bool
		cancelCh      chan struct{} // closed to signal cancellation
	)

	for {
		// Launch recheck if idle and work pending
		if curDone == nil && launchNextRun {
			cancelCh = make(chan struct{})
			go m.runRecheck(nextDone, cancelCh)

			curDone, nextDone = nextDone, make(chan struct{})
			launchNextRun = false
		}

		select {
		case <-m.reqRecheckCh:
			// New block arrived - schedule recheck
			if curDone != nil && cancelCh != nil {
				// Recheck in progress - cancel it (work is stale)
				close(cancelCh)
				cancelCh = nil
			}
			launchNextRun = true
			m.recheckDoneCh <- nextDone

		case <-curDone:
			// Recheck finished
			curDone = nil
			cancelCh = nil

		case <-m.shutdownCh:
			// cancel and wait for in-flight recheck
			if curDone != nil {
				if cancelCh != nil {
					close(cancelCh)
				}
				<-curDone
			}
			close(nextDone)
			return
		}
	}
}

// runRecheck performs the actual recheck work. It holds the write lock for the
// entire duration, iterates through all txs, runs them through the ante handler,
// and removes any that fail (plus dependent txs with higher sequences).
func (m *RecheckMempool) runRecheck(done chan struct{}, cancelled <-chan struct{}) {
	defer close(done)
	start := time.Now()
	txsRemoved := 0
	defer func() {
		recheckDuration.Record(context.Background(), float64(time.Since(start).Milliseconds()),
			metric.WithAttributes(attribute.Int("txs_removed", txsRemoved)))
	}()

	// Hold write lock for entire recheck operation (like legacypool's runReorg)
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx, err := m.getCtx()
	if err != nil {
		m.logger.Error("failed to get context for recheck", "err", err)
		return
	}

	failedAtSequence := make(map[string]uint64)
	removeTxs := make([]sdk.Tx, 0)

	cc, _ := ctx.CacheContext()

	// Use underlying pool's Select (we already hold the lock)
	iter := m.ExtMempool.Select(ctx, nil)
	for iter != nil {
		if isCancelled(cancelled) {
			m.logger.Debug("recheck cancelled - new block arrived")
			return
		}

		txn := iter.Tx()
		if txn == nil {
			break
		}

		signerSeqs, err := m.extractSignerSequences(txn)
		if err != nil {
			m.logger.Error("failed to extract signer sequences", "err", err)
			iter = iter.Next()
			continue
		}

		invalidTx := false
		for _, sig := range signerSeqs {
			if failedSeq, ok := failedAtSequence[sig.account]; ok && failedSeq < sig.seq {
				invalidTx = true
				break
			}
		}

		if !invalidTx {
			txCtx, writeCache := cc.CacheContext()
			newCtx, err := m.anteHandler(txCtx, txn, false)
			if err == nil {
				writeCache()
				cc = newCtx
			} else {
				invalidTx = true
			}
		}

		if invalidTx {
			removeTxs = append(removeTxs, txn)
			for _, sig := range signerSeqs {
				if existing, ok := failedAtSequence[sig.account]; !ok || existing > sig.seq {
					failedAtSequence[sig.account] = sig.seq
				}
			}
		}

		iter = iter.Next()
	}

	if isCancelled(cancelled) {
		m.logger.Debug("recheck cancelled before removal - new block arrived")
		return
	}

	// Use underlying pool's Remove (we already hold the lock)
	for _, txn := range removeTxs {
		if err := m.ExtMempool.Remove(txn); err != nil {
			m.logger.Error("failed to remove tx during recheck", "err", err)
		}
	}
	txsRemoved = len(removeTxs)
}

// extractSignerSequences extracts account addresses and sequences from a tx.
func (m *RecheckMempool) extractSignerSequences(txn sdk.Tx) ([]signerSequence, error) {
	sigTx, ok := txn.(authsigning.SigVerifiableTx)
	if !ok {
		return nil, fmt.Errorf(
			"tx does not implement %T",
			(*authsigning.SigVerifiableTx)(nil),
		)
	}

	sigs, err := sigTx.GetSignaturesV2()
	if err != nil {
		return nil, err
	}

	signerSeqs := make([]signerSequence, 0, len(sigs))
	for _, sig := range sigs {
		signerSeqs = append(signerSeqs, signerSequence{
			account: sig.PubKey.Address().String(),
			seq:     sig.Sequence,
		})
	}

	return signerSeqs, nil
}

// isCancelled checks if the cancellation channel has been closed.
func isCancelled(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
