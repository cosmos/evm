package queue

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gammazero/deque"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/telemetry"
)

// insertItem is an item in the queue that contains the user data (Tx) along
// with a subscription that the user is using to wait on the response from the
// insert.
type insertItem[Tx any] struct {
	tx  *Tx
	sub chan<- error
}

// Queue asynchronously inserts batches of txs in FIFO order.
type Queue[Tx any] struct {
	// queue is a queue of Tx to be processed. Tx's are pushed onto the back, and
	// popped from the front, FIFO.
	queue deque.Deque[insertItem[Tx]]
	lock  sync.RWMutex

	// signal signals that there are Tx's available in the queue. Consumers of
	// the queue should wait on this channel after they have popped all txs off
	// the queue, to know when there are new txs available.
	signal chan struct{}

	// insert inserts a batch of Tx's into the underlying mempool
	insert func(txs []*Tx) []error

	// maxSize is the max amount of Tx's that can be in the queue before
	// rejecting new additions
	maxSize int

	logger log.Logger
	done   chan struct{}
}

var ErrQueueFull = errors.New("queue full")

// New creates a new queue.
func New[Tx any](insert func(txs []*Tx) []error, maxSize int, logger log.Logger) *Queue[Tx] {
	iq := &Queue[Tx]{
		insert:  insert,
		maxSize: maxSize,
		logger:  logger,
		signal:  make(chan struct{}, 1),
		done:    make(chan struct{}),
	}

	go iq.loop()
	return iq
}

// Push enqueues a Tx's to eventually be inserted. Returns a channel that will
// have an error pushed to it if an error occurs inserting the Tx.
func (iq *Queue[Tx]) Push(tx *Tx) <-chan error {
	sub := make(chan error, 1)

	if tx == nil {
		// TODO: when do we expect this to happen?
		close(sub)
		return sub
	}
	if iq.isFull() {
		sub <- ErrQueueFull
		close(sub)
		return sub
	}

	iq.lock.Lock()
	iq.queue.PushBack(insertItem[Tx]{tx: tx, sub: sub})
	iq.lock.Unlock()

	// signal that there are Tx's available
	select {
	case iq.signal <- struct{}{}:
	default:
	}

	return sub
}

// loop is the main loop of the Queue. This will pop Tx's off the front of the
// queue and try to insert them.
func (iq *Queue[Tx]) loop() {
	for {
		iq.lock.RLock()
		numTxsAvailable := iq.queue.Len()
		iq.lock.RUnlock()

		telemetry.SetGauge(float32(numTxsAvailable), "expmempool_inserter_queue_size")

		// if nothing is available, wait for new Tx's to become available
		// before checking again
		if numTxsAvailable == 0 {
			if iq.waitForNewTxs() {
				continue
			}
			return
		}

		var (
			subscriptions []chan<- error
			toInsert      []*Tx
		)

		iq.lock.Lock()
		for item := range iq.queue.IterPopFront() {
			if item.tx == nil {
				close(item.sub)
				continue
			}

			toInsert = append(toInsert, item.tx)
			subscriptions = append(subscriptions, item.sub)
		}
		iq.lock.Unlock()

		errs := iq.insertTxs(toInsert)

		// push any potential errors out to subscribers
		for i, err := range errs {
			subscriptions[i] <- err
			close(subscriptions[i])
		}

		// check if we have been told to cancel, if not, check for more Tx's to insert
		select {
		case <-iq.done:
			return
		default:
			continue
		}
	}
}

// waitForNewTxs blocks and waits for new txs to become available and returns
// true if that happens, or false if we have cancelled before then.
func (iq *Queue[Tx]) waitForNewTxs() bool {
	select {
	case <-iq.done:
		return false
	case <-iq.signal:
		// new txs available
		return true
	}
}

// insertTxs inserts Tx's, returning any errors that have occurred.
func (iq *Queue[Tx]) insertTxs(txs []*Tx) []error {
	defer func(t0 time.Time) {
		telemetry.MeasureSince(t0, "expmempool_inserter_add") //nolint:staticcheck
	}(time.Now())

	errs := iq.insert(txs)
	if len(errs) != len(txs) {
		panic(fmt.Errorf("expected a %d errors from insert but instead got %d", len(txs), len(errs)))
	}
	return errs
}

// isFull returns true if the queue is at capacity and cannot accept anymore
// Tx's, false otherwise.
func (iq *Queue[Tx]) isFull() bool {
	iq.lock.RLock()
	defer iq.lock.RUnlock()
	return iq.queue.Len() >= iq.maxSize
}

// Close stops the main loop of the queue.
func (iq *Queue[Tx]) Close() {
	close(iq.done)
}
