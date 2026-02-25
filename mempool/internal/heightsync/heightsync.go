package heightsync

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/metrics"
)

var (
	// hsTimeout measures the number of times Get timed out before the value
	// at a height was marked complete (partial or no results may be returned).
	hsTimeout = metrics.NewRegisteredMeter("height_sync/timeout", nil)

	// hsComplete measures the number of times Get returned after the value
	// at a height was fully populated (EndCurrentHeight was called).
	hsComplete = metrics.NewRegisteredMeter("height_sync/complete", nil)

	// hsHeightBehind measures the number of times Get was called for a height
	// that the HeightSync had not yet reached.
	hsHeightBehind = metrics.NewRegisteredMeter("height_sync/heightbehind", nil)

	// hsDuration is the total time callers of Get spend from invocation to
	// return.
	hsDuration = metrics.NewRegisteredTimer("height_sync/duration", nil)
)

// HeightSync synchronizes access to a per-height tx store for mempool
// implementations.
//
// At every new block height, mempools need to revalidate (recheck) their
// transactions against the latest chain state. This rechecking happens
// asynchronously — txs are validated one by one and pushed into a store as
// they pass. Meanwhile, block proposers may need to read from that store to
// build the next block.
//
// HeightSync solves this coordination problem. It holds a Store per height
// that producers populate via Do (e.g. during rechecking), and that consumers
// read via GetStore. GetStore blocks until either all operations for the
// requested height are complete (EndCurrentHeight is called) or the caller's
// context times out, whichever comes first. This lets block builders wait for
// the full set of rechecked txs when time permits, while still returning
// partial results under time pressure rather than holding a lock on the
// mempool itself.
type HeightSync[Store any] struct {
	// currentHeight is the height that the value is currently managed for
	currentHeight *big.Int

	// reset is a function that returns a new store, called at every new height
	reset func() *Store

	// store is the current Store that operations are happening on via Do and will
	// be returned via Get until a new height is started via StartNewHeight
	store *Store

	// heightChanged is closed when currentHeight is no longer being worked
	heightChanged chan struct{}

	// done is closed when no more operations will happen on the value at
	// currentHeight
	done chan struct{}

	// mu protects all of the above fields; it does not protect internal
	// fields of the value V itself
	mu sync.RWMutex
}

// New creates a new HeightSync starting at the given height.
func New[Store any](startHeight *big.Int, reset func() *Store) *HeightSync[Store] {
	hs := &HeightSync[Store]{
		currentHeight: startHeight,
		reset:         reset,
		store:         reset(),
		heightChanged: make(chan struct{}),
		done:          make(chan struct{}),
	}
	// initial height is considered immediately done
	hs.EndCurrentHeight()
	return hs
}

// StartNewHeight resets the HeightSync for a new height, overwriting the
// previous Store with a fresh Store via the reset fn.
func (hs *HeightSync[Store]) StartNewHeight(height *big.Int) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	// ensure that the last height that was started has ended before starting a
	// new height
	if !hs.isHeightEnded() {
		panic(fmt.Errorf("height %s not ended before starting new height %s", hs.currentHeight.String(), height.String()))
	}

	// create new Store for this height
	hs.currentHeight = new(big.Int).Set(height)
	hs.store = hs.reset()

	// close old channel and create new one to wake up consumers
	oldChan := hs.heightChanged
	hs.heightChanged = make(chan struct{})
	hs.done = make(chan struct{})
	close(oldChan)
}

// EndCurrentHeight marks the current heights operations as complete. This
// should be called when no more operations on the value via Do will be happen
// for this height.
//
// If operations are not marked as complete, callers of Get must wait for a
// context timeout in order to access the Store at this height.
func (hs *HeightSync[Store]) EndCurrentHeight() {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	if hs.isHeightEnded() {
		panic(fmt.Errorf("height %s already ended", hs.currentHeight.String()))
	}
	close(hs.done)
}

// isHeightDone returns an true if the currentHeight has been ended, false
// otherwise.
func (hs *HeightSync[Store]) isHeightEnded() bool {
	select {
	case <-hs.done:
		// reading from a closed channel will provide the zero value of the
		// channel, done is closed if the height has ended
		return true
	default:
		// were not able to get the zero value which means reading blocked and
		// the done channel was not closed via EndCurrentHeight
		return false
	}
}

// GetStore returns the store at the given height. If the HeightSync has not yet
// reached the target height, GetStore blocks until the height is reached or the
// context expires. If the height is reached, GetStore waits for EndCurrentHeight
// to be called (or for the context to expire) before returning.
func (hs *HeightSync[Store]) GetStore(ctx context.Context, height *big.Int) *Store {
	genesis := big.NewInt(0)

	start := time.Now()
	defer func(t0 time.Time) { hsDuration.UpdateSince(t0) }(start)

	for {
		hs.mu.RLock()

		cmp := hs.currentHeight.Cmp(height)

		// should never see a situation where the HeightSync is ahead of
		// the caller
		if cmp > 0 {
			defer hs.mu.RUnlock() // defer unlock since the panic will read
			panic(fmt.Errorf("HeightSync.Get called for height %d, but current height is %d; cannot serve requests in the past", height, hs.currentHeight))
		}

		// if we're at the target height, wait for completion or timeout
		if cmp == 0 {
			value := hs.store
			done := hs.done
			hs.mu.RUnlock()

			// at genesis, no completion signal will arrive, so return the
			// value immediately
			if hs.currentHeight.Cmp(genesis) == 0 {
				return value
			}

			// wait for EndCurrentHeight to signal that all operations on
			// the value are complete, or for the context to expire
			select {
			case <-done:
				hsComplete.Mark(1)
			case <-ctx.Done():
				hsTimeout.Mark(1)
			}
			return value
		}

		// current height is behind target, we cannot return the Store at the
		// current height, so we must wait for the height to advance to the
		// callers target height
		heightChangedChan := hs.heightChanged
		hs.mu.RUnlock()

		hsHeightBehind.Mark(1)

		// wait for height to advance or context to timeout
		select {
		case <-heightChangedChan:
			// height changed, loop back to check if we've reached target
			continue
		case <-ctx.Done():
			// caller is done waiting, but we still do not have a Store at the
			// correct height to return, return nil instead
			hsTimeout.Mark(1)
			return nil
		}
	}
}

// Do executes fn with the current store. The store will be non-nil.
func (hs *HeightSync[Store]) Do(fn func(store *Store)) {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	if hs.store == nil {
		return
	}
	fn(hs.store)
}
