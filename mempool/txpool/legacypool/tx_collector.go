package legacypool

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/cosmos/evm/mempool/txpool"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/holiman/uint256"
)

var (
	// collectorTimeout measures the amount of times the collector timed out
	// before being able to serve all collected txs at a height (this may mean
	// partial txs returned, or none at all).
	collectorTimeout = metrics.NewRegisteredMeter("collector/timeout", nil)

	// collectorComplete measures the amount of times the collector was able to
	// serve all txs that would be collected for a height in a Collect request.
	collectorComplete = metrics.NewRegisteredMeter("collector/complete", nil)

	// collectorHeightBehind measures the amount of times the collector
	// received a request to collect txs for a height and the collector was >=
	// 1 height behind the target height.
	collectorHeightBehind = metrics.NewRegisteredMeter("collector/heightbehind", nil)

	// txsCollected is the total amount of txs returned by Collect.
	txsCollected = metrics.NewRegisteredMeter("collector/txscollected", nil)

	// collectorWaitDuration is the amount of time callers of Collect spend
	// waiting to get a response (via timeout or completion).
	collectorWaitDuration = metrics.NewRegisteredTimer("collector/waittime", nil)

	// collectorWaitDuration is the amount of time callers of Collect spend
	// waiting to get a response (via timeout or completion).
	collectorRemoveDuraiton = metrics.NewRegisteredTimer("collector/removetime", nil)
)

// txCollector collects txs at a height given height.
type txCollector struct {
	// currentHeight is the height that txs are currently being collected for
	currentHeight *big.Int

	// txs is the set of txs collected at currentHeight
	txs *txs

	// heightChanged is closed when currentHeight is no longer being worked
	heightChanged chan struct{}

	// noMoreTxs is closed when no more txs will arrive for currentHeight
	noMoreTxs chan struct{}

	// mu protects all of the above values, it does not protect internal values
	// to txs
	mu sync.RWMutex
}

// newTxCollector creates a new tx collector.
func newTxCollector(startHeight *big.Int) *txCollector {
	return &txCollector{
		currentHeight: startHeight,
		heightChanged: make(chan struct{}),
		noMoreTxs:     make(chan struct{}),
		txs:           newTxs(),
	}
}

// StartNewHeight begins tracking a new height and returns a completion
// callback. StartNewHeight should be called when a new block is seen, before
// any txs are added to the pending set for this height.
//
// The completion callback should be called when no more txs for this height
// will be added to the set.
func (c *txCollector) StartNewHeight(height *big.Int) func() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create new builder for this height
	c.currentHeight = new(big.Int).Set(height)
	c.txs = newTxs()

	// Close old channel and create new one to wake up consumers
	oldChan := c.heightChanged
	c.heightChanged = make(chan struct{})
	c.noMoreTxs = make(chan struct{})
	close(oldChan)

	// Return completion callback
	return func() {
		close(c.noMoreTxs)
	}
}

// Collect collects txs in the collector at a height. If this height has not
// been reached by the collector, it will wait until the context times out or
// the height is reached.
func (c *txCollector) Collect(ctx context.Context, height *big.Int, filter txpool.PendingFilter) map[common.Address][]*txpool.LazyTransaction {
	genesis := big.NewInt(0)

	start := time.Now()
	for {
		c.mu.RLock()

		cmp := c.currentHeight.Cmp(height)

		// Should never see a situation where the collector has a higher hight
		// than the callers
		if cmp > 0 {
			defer c.mu.RUnlock() // Defer unlock since the panic will read
			panic(fmt.Errorf("collector received collect request at height %d, but collector is at height %d, cannot serve requests in the past", height, c.currentHeight))
		}

		// If we're at the target height, wait for completion or timeout
		if cmp == 0 {
			txs := c.txs
			done := c.noMoreTxs
			c.mu.RUnlock()

			// If we're at genesis, we will not get the signal from the mempool
			// that no more txs will arrive, since the mempool is not going to
			// be reset at this height, therefore we simply return any txs that
			// have been added to the collector at this point
			if c.currentHeight.Cmp(genesis) == 0 {
				collectorWaitDuration.UpdateSince(start)
				ts := txs.Get(filter)
				return ts
			}

			// Not at genesis, we must wait for the mempool to signal that it
			// has been reset fully at this height, or we have timed out
			// waiting for that to occur and we return incomplete results.
			select {
			case <-done:
				collectorComplete.Mark(1)
			case <-ctx.Done():
				collectorTimeout.Mark(1)
			}

			collectorWaitDuration.UpdateSince(start)
			ts := txs.Get(filter)
			return ts
		}

		// Current height is behind target - capture the channel before unlocking
		heightChangedChan := c.heightChanged
		c.mu.RUnlock()
		collectorHeightBehind.Mark(1)

		// Wait for height to advance, context to timeout, or manager to close
		select {
		case <-heightChangedChan:
			// Height changed, loop back to check if we've reached target
			continue
		case <-ctx.Done():
			// Timeout before reaching target height - return nil
			collectorTimeout.Mark(1)
			collectorWaitDuration.UpdateSince(start)
			return nil
		}
	}
}

// AddList adds a list of txs to the collector.
func (c *txCollector) AddList(addr common.Address, list *list) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.txs == nil {
		return
	}
	c.txs.Add(addr, list.Flatten())
}

// AddTx adds a single tx to the collector.
func (c *txCollector) AddTx(addr common.Address, tx *types.Transaction) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.txs == nil {
		return
	}
	c.txs.Add(addr, []*types.Transaction{tx})
}

// RemoveTx removes a tx from the collector.
func (c *txCollector) RemoveTx(addr common.Address, tx *types.Transaction) {
	defer func(t0 time.Time) { collectorRemoveDuraiton.UpdateSince(t0) }(time.Now())

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.txs == nil {
		return
	}
	c.txs.Remove(addr, tx)
}

// txs is a set of transactions at a height that can be added to or removed
// from.
type txs struct {
	txs map[common.Address]types.Transactions

	// lookup provides a fast lookup to determine if a tx is in the set or not.
	lookup map[common.Hash]struct{}

	mu sync.RWMutex
}

// newTxs creates a new txs set.
func newTxs() *txs {
	return &txs{
		txs:    make(map[common.Address]types.Transactions),
		lookup: make(map[common.Hash]struct{}),
	}
}

// Get returns the current set of txs.
func (t *txs) Get(filter txpool.PendingFilter) map[common.Address][]*txpool.LazyTransaction {
	// Do not support blob txs
	if filter.OnlyBlobTxs {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Convert the new uint256.Int types to the old big.Int ones used by the legacy pool
	var (
		minTipBig  *big.Int
		baseFeeBig *big.Int
	)
	if filter.MinTip != nil {
		minTipBig = filter.MinTip.ToBig()
	}
	if filter.BaseFee != nil {
		baseFeeBig = filter.BaseFee.ToBig()
	}

	numSelected := 0
	pending := make(map[common.Address][]*txpool.LazyTransaction, len(t.txs))

	for addr, txs := range t.txs {
		sort.Sort(types.TxByNonce(txs))

		// Filter by minimum tip if configured
		if minTipBig != nil {
			for i, tx := range txs {
				if tx.EffectiveGasTipIntCmp(minTipBig, baseFeeBig) < 0 {
					txs = txs[:i]
					break
				}
			}
		}

		// Convert to lazy transactions
		if len(txs) > 0 {
			lazies := make([]*txpool.LazyTransaction, len(txs))
			for i, tx := range txs {
				lazies[i] = &txpool.LazyTransaction{
					Hash:      tx.Hash(),
					Tx:        tx,
					Time:      tx.Time(),
					GasFeeCap: uint256.MustFromBig(tx.GasFeeCap()),
					GasTipCap: uint256.MustFromBig(tx.GasTipCap()),
					Gas:       tx.Gas(),
					BlobGas:   tx.BlobGas(),
				}
			}
			numSelected += len(lazies)
			pending[addr] = lazies
		}
	}

	txsCollected.Mark(int64(numSelected))
	return pending
}

// add adds txs to the current set.
func (t *txs) Add(addr common.Address, txs types.Transactions) {
	t.mu.Lock()
	defer t.mu.Unlock()

	toAdd := make([]*types.Transaction, 0, len(txs))
	for _, tx := range txs {
		if _, exists := t.lookup[tx.Hash()]; exists {
			continue
		}
		toAdd = append(toAdd, tx)
	}

	if existing, ok := t.txs[addr]; ok {
		t.txs[addr] = append(existing, toAdd...)
	} else {
		t.txs[addr] = toAdd
	}
}

// RemoveTx removes a tx for an address from the current set.
func (t *txs) Remove(addr common.Address, tx *types.Transaction) {
	t.mu.Lock()
	defer t.mu.Unlock()

	defer delete(t.lookup, tx.Hash())

	txs, ok := t.txs[addr]
	if !ok {
		return
	}

	// Find and remove the tx by nonce
	nonce := tx.Nonce()
	for i := 0; i < len(txs); i++ {
		if txs[i].Nonce() == nonce {
			// Swap with last element and truncate
			txs[i] = txs[len(txs)-1]
			t.txs[addr] = txs[:len(txs)-1]
			return
		}
	}
}
