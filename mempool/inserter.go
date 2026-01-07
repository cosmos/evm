package mempool

import (
	"fmt"
	"time"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/telemetry"
	"github.com/cosmos/evm/mempool/txpool"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/gammazero/deque"
)

const (
	noTxsAvailableSleepDuration = 250 * time.Millisecond
)

type insertQueue struct {
	queue deque.Deque[*ethtypes.Transaction]
	pool  txpool.SubPool

	logger log.Logger
	done   chan struct{}
}

func newInsertQueue(pool txpool.SubPool, logger log.Logger) *insertQueue {
	iq := &insertQueue{
		pool:   pool,
		logger: logger,
		done:   make(chan struct{}),
	}

	go iq.loop()
	return iq
}

func (iq *insertQueue) Push(tx *ethtypes.Transaction) {
	if tx == nil {
		return
	}
	iq.queue.PushBack(tx)
}

func (iq *insertQueue) loop() {
	for {
		select {
		case <-iq.done:
			return
		default:
			numTxsAvailable := iq.queue.Len()
			telemetry.SetGauge(float32(numTxsAvailable), "expmempool_inserter_queue_size") //nolint:staticcheck // TODO: switch to OpenTelemetry

			if numTxsAvailable > 0 {
				if tx := iq.queue.PopFront(); tx != nil {
					start := time.Now()
					errs := iq.pool.Add([]*ethtypes.Transaction{tx}, false)
					telemetry.MeasureSince(start, "expmempool_inserter_add") //nolint:staticcheck // TODO: switch to OpenTelemetry
					if len(errs) != 1 {
						panic(fmt.Errorf("expected a single error when compacting evm tx add errors"))
					}
					if errs[0] != nil {
						iq.logger.Error("error inserting evm tx into pool", "tx_hash", tx.Hash(), "err", errs[0])
					}
				}
				if numTxsAvailable-1 > 0 {
					// there are still txs available, try and insert immediately again
					continue
				}
			}

			// no txs available, sleep then check again
			select {
			case <-iq.done:
				return
			case <-time.After(noTxsAvailableSleepDuration):
			}
		}
	}
}

func (iq *insertQueue) Close() {
	close(iq.done)
}
