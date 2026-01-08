package mempool

import (
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

// insertQueue allows for txs to be pushed into a pool asynchronously.
type insertQueue struct {
	// queue is a queue of txs to be inserted into the pool. txs are pushed
	// onto the back, and popped from the from, FIFO.
	queue deque.Deque[*ethtypes.Transaction]
	lock  sync.RWMutex

	// signal signals that there are txs available in the queue. Consumers of
	// the queue should wait on this channel after they have popped all txs off
	// the queue, to know when there are new txs available.
	signal chan struct{}

	// pool is the txPool that txs will be added to.
	pool TxPool

	logger log.Logger
	done   chan struct{}
}

// newInsertQueue creates a new insertQueue
func newInsertQueue(pool TxPool, logger log.Logger) *insertQueue {
	iq := &insertQueue{
		pool:   pool,
		logger: logger,
		signal: make(chan struct{}, 1),
		done:   make(chan struct{}),
	}

	go iq.loop()
	return iq
}

// Push enqueues a tx to eventually be added to the pool.
func (iq *insertQueue) Push(tx *ethtypes.Transaction) {
	if tx == nil {
		return
	}

	iq.lock.Lock()
	iq.queue.PushBack(tx)
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
			tx := iq.queue.PopFront()
			iq.lock.Unlock()

			if tx != nil {
				iq.addTx(tx)
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

// addTx adds a tx to the pool, logging an error if any occurred.
func (iq *insertQueue) addTx(tx *ethtypes.Transaction) {
	defer func(t0 time.Time) {
		telemetry.MeasureSince(t0, "expmempool_inserter_add") //nolint:staticcheck
	}(time.Now())

	errs := iq.pool.Add([]*ethtypes.Transaction{tx}, false)
	if len(errs) != 1 {
		panic(fmt.Errorf("expected a single error when compacting evm tx add errors"))
	}
	if errs[0] != nil {
		iq.logger.Error("error inserting evm tx into pool", "tx_hash", tx.Hash(), "err", errs[0])
	}
}

// Close stops the main loop of the insert queue.
func (iq *insertQueue) Close() {
	close(iq.done)
}
