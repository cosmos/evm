package mempool

import (
	"errors"
	"fmt"
	"sync"
	"time"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/gammazero/deque"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/telemetry"
)

type TxPool interface {
	Add(txs []*ethtypes.Transaction, sync bool) []error
}

type item struct {
	tx  *ethtypes.Transaction
	sub chan<- error
}

// insertQueue allows for txs to be pushed into a pool asynchronously.
type insertQueue struct {
	// queue is a queue of txs to be inserted into the pool. txs are pushed
	// onto the back, and popped from the from, FIFO.
	queue deque.Deque[item]
	lock  sync.RWMutex

	// signal signals that there are txs available in the queue. Consumers of
	// the queue should wait on this channel after they have popped all txs off
	// the queue, to know when there are new txs available.
	signal chan struct{}

	// pool is the txPool that txs will be added to.
	pool TxPool

	// maxSize is the max amount of txs that can be in the insert queue before
	// rejecting new additions
	maxSize uint64

	logger log.Logger
	done   chan struct{}
}

var (
	ErrInsertQueueFull = errors.New("insert queue full")
)

// newInsertQueue creates a new insertQueue
func newInsertQueue(pool TxPool, maxSize uint64, logger log.Logger) *insertQueue {
	iq := &insertQueue{
		pool:    pool,
		maxSize: maxSize,
		logger:  logger,
		signal:  make(chan struct{}, 1),
		done:    make(chan struct{}),
	}

	go iq.loop()
	return iq
}

// Push enqueues a tx to eventually be added to the pool.
func (iq *insertQueue) Push(tx *ethtypes.Transaction, sub chan<- error) {
	if tx == nil {
		return
	}

	if iq.isFull() {
		if sub != nil {
			sub <- ErrInsertQueueFull
			close(sub)
		}
		return
	}

	iq.lock.Lock()
	iq.queue.PushBack(item{tx: tx, sub: sub})
	iq.lock.Unlock()

	// signal that there are txs available
	select {
	case iq.signal <- struct{}{}:
	default:
	}
}

// loop is the main loop of the insertQueue. This will pop txs off the front of
// the queue and try to add them to the pool.
func (iq *insertQueue) loop() {
	for {
		iq.lock.RLock()
		numTxsAvailable := iq.queue.Len()
		iq.lock.RUnlock()

		telemetry.SetGauge(float32(numTxsAvailable), "expmempool_inserter_queue_size")
		if numTxsAvailable > 0 {
			iq.lock.Lock()
			item := iq.queue.PopFront()
			iq.lock.Unlock()

			if item.tx != nil {
				err := iq.addTx(item.tx)

				// if the item has a subscriber on it, push any errors that
				// occurred to them
				if item.sub != nil {
					item.sub <- err
					close(item.sub)
				}
			}
			if numTxsAvailable-1 > 0 {
				// there are still txs available, try and insert immediately
				// again, unless cancelled
				select {
				case <-iq.done:
					return
				default:
					continue
				}
			}
		}

		// no txs available, block until signaled or done
		select {
		case <-iq.done:
			return
		case <-iq.signal:
			// new txs available
		}
	}
}

// addTx adds a tx to the pool, returning any errors that occurred
func (iq *insertQueue) addTx(tx *ethtypes.Transaction) error {
	defer func(t0 time.Time) {
		telemetry.MeasureSince(t0, "expmempool_inserter_add") //nolint:staticcheck
	}(time.Now())

	errs := iq.pool.Add([]*ethtypes.Transaction{tx}, false)
	if len(errs) != 1 {
		panic(fmt.Errorf("expected a single error when compacting evm tx add errors"))
	}
	return errs[0]
}

// isFull returns true if the insert queue is at capacity and cannot accept
// anymore items, false otherwise.
func (iq *insertQueue) isFull() bool {
	iq.lock.RLock()
	defer iq.lock.RUnlock()
	return iq.queue.Len() >= int(iq.maxSize)
}

// Close stops the main loop of the insert queue.
func (iq *insertQueue) Close() {
	close(iq.done)
}
