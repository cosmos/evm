package legacypool

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"sync"

	"github.com/cosmos/evm/mempool/txpool"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/holiman/uint256"
)

var (
	pendingTimeout        = metrics.NewRegisteredMeter("collector/timeout", nil)
	pendingComplete       = metrics.NewRegisteredMeter("collector/complete", nil)
	pendingTxsAccumulated = metrics.NewRegisteredMeter("collector/txsaccumulated", nil)
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
func (c *txCollector) Collect(ctx context.Context, height *big.Int, minTip *big.Int, baseFee *big.Int) map[common.Address][]*txpool.LazyTransaction {
	for {
		c.mu.RLock()

		cmp := c.currentHeight.Cmp(height)

		// Panic if requesting old height (programming error)
		if cmp > 0 {
			c.mu.RUnlock()
			panic(fmt.Errorf("requested height %d but current height is %d (cannot serve old heights)", height, c.currentHeight))
		}

		// If we're at the target height, wait for completion or timeout
		if cmp == 0 {
			txs := c.txs
			done := c.noMoreTxs
			c.mu.RUnlock()

			// Wait for builder to complete, context to timeout, or manager to close

			select {
			case <-done:
				pendingComplete.Mark(1)
			case <-ctx.Done():
				// Timeout while processing target height - return partial results
				pendingTimeout.Mark(1)
			}
			return txs.Get(minTip, baseFee)
		}

		// Current height is behind target - capture the channel before unlocking
		heightChangedChan := c.heightChanged
		c.mu.RUnlock()

		// Wait for height to advance, context to timeout, or manager to close
		select {
		case <-heightChangedChan:
			// Height changed, loop back to check if we've reached target
			continue
		case <-ctx.Done():
			// Timeout before reaching target height - return nil
			return nil
		}
	}
}

// AddList adds a list of validated txs to the current builder.
func (c *txCollector) AddList(addr common.Address, list *list) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.txs == nil {
		return
	}
	c.txs.Add(addr, list.Flatten())
}

// AddTx adds a single validated tx to the current builder.
func (c *txCollector) AddTx(addr common.Address, tx *types.Transaction) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.txs == nil {
		return
	}
	c.txs.Add(addr, []*types.Transaction{tx})
}

// RemoveTx removes a tx from the current builder.
func (c *txCollector) RemoveTx(addr common.Address, tx *types.Transaction) {
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
	mu  sync.RWMutex
}

// newTxs creates a new txs set.
func newTxs() *txs {
	return &txs{
		txs: make(map[common.Address]types.Transactions),
	}
}

// GetValidPendingTxs returns the current set of validated pending txs.
// This can be called at any time and returns whatever has been validated so far.
func (t *txs) Get(minTip *big.Int, baseFee *big.Int) map[common.Address][]*txpool.LazyTransaction {
	t.mu.Lock()
	defer t.mu.Unlock()

	numSelected := 0
	pending := make(map[common.Address][]*txpool.LazyTransaction, len(t.txs))

	for addr, txs := range t.txs {
		sort.Sort(types.TxByNonce(txs))

		// Filter by minimum tip if configured
		if minTip != nil {
			for i, tx := range txs {
				if tx.EffectiveGasTipIntCmp(minTip, baseFee) < 0 {
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

	pendingTxsAccumulated.Mark(int64(numSelected))
	return pending
}

// add adds txs to the current set.
func (t *txs) Add(addr common.Address, txs types.Transactions) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if existing, ok := t.txs[addr]; ok {
		t.txs[addr] = append(existing, txs...)
	} else {
		t.txs[addr] = txs
	}
}

// RemoveTx removes a tx for an address from the current set.
func (t *txs) Remove(addr common.Address, tx *types.Transaction) {
	t.mu.Lock()
	defer t.mu.Unlock()

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
