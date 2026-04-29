package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gammazero/deque"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("github.com/cosmos/evm/mempool/internal/queue")

var (
	// queueSize is the current number of txs waiting to be inserted into the
	// underlying mempool.
	queueSize metric.Int64Gauge

	// insertDuration is the latency of a batch insert into the underlying
	// mempool.
	insertDuration metric.Float64Histogram
)

func init() {
	var err error
	queueSize, err = meter.Int64Gauge(
		"mempool.inserter.queue_size",
		metric.WithDescription("Number of txs waiting in the inserter queue"),
	)
	if err != nil {
		panic(err)
	}

	insertDuration, err = meter.Float64Histogram(
		"mempool.inserter.add_duration",
		metric.WithDescription("Time to insert a batch of txs into the underlying mempool"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		panic(err)
	}
}

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

	// metricAttrs identifies this queue in emitted metrics.
	metricAttrs metric.MeasurementOption

	done chan struct{}
}

var ErrQueueFull = errors.New("queue full")

// New creates a new queue. name distinguishes this queue's metrics from other
// queue instances (e.g. "evm" vs "cosmos").
func New[Tx any](name string, insert func(txs []*Tx) []error, maxSize int) *Queue[Tx] {
	iq := &Queue[Tx]{
		insert:      insert,
		maxSize:     maxSize,
		signal:      make(chan struct{}, 1),
		done:        make(chan struct{}),
		metricAttrs: metric.WithAttributeSet(attribute.NewSet(attribute.String("pool", name))),
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

		queueSize.Record(context.Background(), int64(numTxsAvailable), iq.metricAttrs)

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
		insertDuration.Record(context.Background(), float64(time.Since(t0).Milliseconds()), iq.metricAttrs)
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
