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

	// hsWaitDuration is the time callers of Get spend waiting for the value
	// to become available (via completion or timeout).
	hsWaitDuration = metrics.NewRegisteredTimer("height_sync/waittime", nil)

	// hsDuration is the total time callers of Get spend from invocation to
	// return.
	hsDuration = metrics.NewRegisteredTimer("height_sync/duration", nil)
)

// HeightSync manages per height access to an instance of a value V.
type HeightSync[V any] struct {
	// currentHeight is the height that the value is currently managed for
	currentHeight *big.Int

	// vFactory is a function that returns a new *V, called at every new height
	vFactory func() *V

	// value is the current *V that operations are happening on via Do and will
	// be returned via Get until a new height is started via StartNewHeight
	value *V

	// heightChanged is closed when currentHeight is no longer being worked
	heightChanged chan struct{}

	// done is closed when no more operations will happen on the value at
	// currentHeight
	done chan struct{}

	// mu protects all of the above fields; it does not protect internal
	// fields of the value V itself
	mu sync.RWMutex
}

// NewHeightSync creates a new HeightSync starting at the given height.
func NewHeightSync[V any](startHeight *big.Int, factory func() *V) *HeightSync[V] {
	return &HeightSync[V]{
		currentHeight: startHeight,
		vFactory:      factory,
		value:         factory(),
		heightChanged: make(chan struct{}),
		done:          make(chan struct{}),
	}
}

// StartNewHeight resets the HeightSync for a new height, overwriting the
// previous value V with a fresh value via the factory.
func (hs *HeightSync[V]) StartNewHeight(height *big.Int) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	// create new value for this height
	hs.currentHeight = new(big.Int).Set(height)
	hs.value = hs.vFactory()

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
// context timeout in order to access the value at this height.
func (hs *HeightSync[V]) EndCurrentHeight() {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	close(hs.done)
}

// GetValue returns the value at the given height. If the HeightSync has not yet
// reached the target height, GetValue blocks until the height is reached or the
// context expires. If the height is reached, GetValue waits for EndCurrentHeight
// to be called (or for the context to expire) before returning.
func (hs *HeightSync[V]) GetValue(ctx context.Context, height *big.Int) *V {
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
			value := hs.value
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

		// current height is behind target, we cannot return the value V at the
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
			// caller is done waiting, but we still do not have a value at the
			// correct height to return, return nil instead
			hsTimeout.Mark(1)
			return nil
		}
	}
}

// Do executes fn with the current value. The value will be non-nil.
func (hs *HeightSync[V]) Do(fn func(v *V)) {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	if hs.value == nil {
		return
	}
	fn(hs.value)
}
