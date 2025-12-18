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
	pendingTimeout        = metrics.NewRegisteredMeter("pendingbuilder/timeout", nil)
	pendingComplete       = metrics.NewRegisteredMeter("pendingbuilder/complete", nil)
	pendingTxsAccumulated = metrics.NewRegisteredMeter("pendingbuilder/txsaccumulated", nil)

	pendingSelectWait    = metrics.NewRegisteredTimer("pendingbuilder/selectwait", nil)
	pendingResetToSelect = metrics.NewRegisteredTimer("pendingbuilder/resettoselect", nil)
)

type processedHeight struct {
	height *big.Int
	done   chan struct{}
}

// pendingBuilder builds a set of pending txs as they are validated.
type pendingBuilder struct {
	minTip    *big.Int
	priceBump uint64

	currentValidPendingTxs map[common.Address]types.Transactions
	mu                     sync.RWMutex

	height *big.Int

	processingHeights chan processedHeight

	resetAt time.Time
	done    chan struct{}
}

// newPendingBuilder creates a new instance of a pendingBuilder.
func newPendingBuilder(minTip *big.Int, height *big.Int, priceBump uint64) *pendingBuilder {
	pb := &pendingBuilder{
		minTip:                 minTip,
		priceBump:              priceBump,
		currentValidPendingTxs: make(map[common.Address]types.Transactions),
		height:                 height,
		done:                   make(chan struct{}),
		processingHeights:      make(chan processedHeight, 100),
	}

	return pb
}

// ValidPendingTxs gets the current set of valid pending txs from the builder.
// This will block until either the supplied context times out, or the height
// is marked as having been fully validated.
func (pb *pendingBuilder) ValidPendingTxs(ctx context.Context, height *big.Int, baseFee *big.Int) map[common.Address][]*txpool.LazyTransaction {
	if !pb.AdvanceToTarget(ctx, height) {
		return nil
	}

	pb.mu.Lock()
	defer pb.mu.Unlock()

	numSelected := 0
	pending := make(map[common.Address][]*txpool.LazyTransaction, len(pb.currentValidPendingTxs))
	for addr, txs := range pb.currentValidPendingTxs {
		sort.Sort(types.TxByNonce(txs)) // == list.Flatten()

		if pb.minTip != nil {
			for i, tx := range txs {
				if tx.EffectiveGasTipIntCmp(pb.minTip, baseFee) < 0 {
					txs = txs[:i]
					break
				}
			}
		}
		if len(txs) > 0 {
			lazies := make([]*txpool.LazyTransaction, len(txs))
			for i := 0; i < len(txs); i++ {
				lazies[i] = &txpool.LazyTransaction{
					// pool field is unused
					Hash:      txs[i].Hash(),
					Tx:        txs[i],
					Time:      txs[i].Time(),
					GasFeeCap: uint256.MustFromBig(txs[i].GasFeeCap()),
					GasTipCap: uint256.MustFromBig(txs[i].GasTipCap()),
					Gas:       txs[i].Gas(),
					BlobGas:   txs[i].BlobGas(),
				}
			}
			numSelected += len(lazies)
			pending[addr] = lazies
		}
	}
	pendingTxsAccumulated.Mark(int64(numSelected))
	return pending
}

// AddList adds a list of txs for an address that have been validated to the
// current pending set.
func (pb *pendingBuilder) AddList(addr common.Address, list *list) {
	pb.add(addr, list.Flatten())
}

// AddTx adds a single tx for an address that has been validated to the
// current pending set.
func (pb *pendingBuilder) AddTx(addr common.Address, tx *types.Transaction) {
	pb.add(addr, []*types.Transaction{tx})
}

// add adds validated txs to the current pending set.
func (pb *pendingBuilder) add(addr common.Address, txs types.Transactions) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if _, ok := pb.currentValidPendingTxs[addr]; !ok {
		pb.currentValidPendingTxs[addr] = txs
		return nil
	}

	pb.currentValidPendingTxs[addr] = append(pb.currentValidPendingTxs[addr], txs...)
	return nil
}

// RemoveTx removes a tx for an address from the current pending set.
func (pb *pendingBuilder) RemoveTx(addr common.Address, remove *types.Transaction) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	txs, ok := pb.currentValidPendingTxs[addr]
	if !ok {
		return
	}

	// TODO: maybe use the SortedMap for faster removals if this is a bottleneck?
	nonce := remove.Nonce()
	for i := 0; i < len(txs); i++ {
		if txs[i].Nonce() == nonce {
			txs[i] = txs[len(txs)-1]
			txs = txs[:len(txs)-1]
			return
		}
	}
}

func (pb *pendingBuilder) AdvanceToTarget(ctx context.Context, target *big.Int) bool {
	for {
		pb.mustBeBelowHeight(target)
		fmt.Printf("pending builder at %d while target is %d\n", pb.height, target)

		// as new heights are processed, info about them (height and a signal
		// for completion) will be pushed onto pendingHeights.
		select {
		case <-pb.done:
			return false
		case <-ctx.Done():
			// timeout but we never even saw a new height trying to be
			// processed, this is an issue, panic
			panic("context timeout without ever seeing height being processed")
		case latest := <-pb.processingHeights:
			pb.height = latest.height
			fmt.Printf("popped off height %d, this is either in progress or done\n", pb.height)
			if !pb.isHeightProcessed(ctx, latest) {
				// we have advanced to height latest.height, but it did not
				// process, we cannot call the pending builder advanced to
				// target yet
				fmt.Printf("have not reached target height, waiting for next height to start processing\n", pb.height)
				return false
			}
			if pb.isAtHeight(target) {
				// we have advanced to height latest.height and processed it,
				// if this is the target height, we have officially advanced to
				// this height
				fmt.Printf("have reached target height of %d, returning what has been checked so far\n", target)
				return true
			}
		}
	}
}

func (pb *pendingBuilder) isHeightProcessed(ctx context.Context, processedHeight processedHeight) bool {
	select {
	case <-pb.done:
		return false
	case <-ctx.Done():
		// if we are working on checking this height, but have timed
		// out, we should return any pending txs that have been added
		// to the builder at this point. if we are not working on this
		// height, then we should not return txs
		fmt.Printf("timeout waiting for height %d to be done\n", processedHeight.height)
		defer pendingTimeout.Mark(1)
		return true
	case <-processedHeight.done:
		fmt.Printf("height %d done\n", processedHeight.height)
		return true
	}
}

func (pb *pendingBuilder) isAtHeight(target *big.Int) bool {
	return pb.height.Cmp(target) == 0
}

func (pb *pendingBuilder) mustBeBelowHeight(target *big.Int) {
	if pb.height.Cmp(target) > 0 {
		panic(fmt.Errorf("pending builder height %d while target is at %d", pb.height, target))
	}
}

// Reset resets the internal state of the pendingBuilder to a new height. This
// must be called when a new block is seen, before any txs have been validated
// on top of that blocks state.
func (pb *pendingBuilder) Reset(height *big.Int) func() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.currentValidPendingTxs = make(map[common.Address]types.Transactions)
	pb.resetAt = time.Now() // for metrics
	return pb.ProcessHeight(height)
}

// Close terminates the pending builder
func (pb *pendingBuilder) Close() {
	close(pb.done)
}

func (pb *pendingBuilder) mustBeBelowOrAt(targetHeight *big.Int) {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	if pb.height.Cmp(targetHeight) > 0 {
		panic(fmt.Errorf("pending builder at height %d, but target height is %d", pb.height, targetHeight))
	}
}

func (pb *pendingBuilder) ProcessHeight(height *big.Int) func() {
	receipt := processedHeight{
		height: height,
		done:   make(chan struct{}),
	}

	select {
	// if pushing onto this channel blocks, then there has been no consumer on
	// the other end, this node is likely not proposing blocks in this case,
	// and we dont care about properly marking the receipt as done, so we
	// return a mock fn instead.
	case pb.processingHeights <- receipt:
		return func() { close(receipt.done) }
	default:
		return func() {}
	}
}
