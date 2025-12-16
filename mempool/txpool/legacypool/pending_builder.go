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
	"github.com/holiman/uint256"
)

// pendingBuilder builds a set of pending txs as they are validated.
type pendingBuilder struct {
	in chan validatedTxs

	minTip    *big.Int
	priceBump uint64

	currentValidPendingTxs map[common.Address]types.Transactions
	height                 *big.Int
	heightValidated        chan struct{}
	mu                     sync.RWMutex

	done chan struct{}
}

// newPendingBuilder creates a new instance of a pendingBuilder.
func newPendingBuilder(minTip *big.Int, priceBump uint64) *pendingBuilder {
	pb := &pendingBuilder{
		in:                     make(chan validatedTxs),
		minTip:                 minTip,
		priceBump:              priceBump,
		currentValidPendingTxs: make(map[common.Address]types.Transactions),
		done:                   make(chan struct{}),
	}
	go pb.loop()
	return pb
}

// ValidPendingTxs gets the current set of valid pending txs from the builder.
// This will block until either the supplied context times out, or the height
// is marked as having been fully validated.
func (pb *pendingBuilder) ValidPendingTxs(ctx context.Context, height *big.Int, baseFee *big.Int) map[common.Address][]*txpool.LazyTransaction {
	// ensure we are working on the height requested
	pb.mustHaveHeight(height)

	// wait for two cases:
	// 1. the user has set some defined timeout in the context that we must
	// respect. so if we time out on context then we will return all of the
	// things that have been validated up to this point.
	// 2. we have gotten the signal that all txs have been validated at this height and we can now return.
	select {
	case <-pb.done:
		return nil
	case <-ctx.Done():
	case <-pb.heightValidated:
	}

	pb.mu.Lock()
	defer pb.mu.Unlock()
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
			pending[addr] = lazies
		}
	}
	return pending
}

// validatedTxs is a wrapper around a set of txs and their from address.
type validatedTxs struct {
	txs     types.Transactions
	address common.Address
}

// AddList adds a list of txs for an address that have been validated to the
// current pending set.
func (pb *pendingBuilder) AddList(addr common.Address, list *list) {
	pb.in <- validatedTxs{address: addr, txs: list.Flatten()}
}

// AddTx adds a single tx for an address that has been validated to the
// current pending set.
func (pb *pendingBuilder) AddTx(addr common.Address, tx *types.Transaction) {
	pb.in <- validatedTxs{address: addr, txs: []*types.Transaction{tx}}
}

// loop is the main event loop of the pendingBuilder, adding validated txs to
// its internal pending set as they are pushed the input chan.
func (pb *pendingBuilder) loop() {
	for {
		select {
		case <-pb.done:
			return
		case validatedTxs := <-pb.in:
			pb.add(validatedTxs)
		}
	}
}

// add adds validated txs to the current pending set.
func (pb *pendingBuilder) add(validatedTxs validatedTxs) error {
	pb.mustHaveHeight(pb.height)

	pb.mu.Lock()
	defer pb.mu.Unlock()

	if _, ok := pb.currentValidPendingTxs[validatedTxs.address]; !ok {
		pb.currentValidPendingTxs[validatedTxs.address] = validatedTxs.txs
		return nil
	}

	pb.currentValidPendingTxs[validatedTxs.address] = append(pb.currentValidPendingTxs[validatedTxs.address], validatedTxs.txs...)
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

// Reset resets the internal state of the pendingBuilder to a new height. This
// must be called when a new block is seen, before any txs have been validated
// on top of that blocks state.
func (pb *pendingBuilder) Reset(height *big.Int) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.height = height
	pb.currentValidPendingTxs = make(map[common.Address]types.Transactions)
	pb.heightValidated = make(chan struct{})
}

// MarkHeightValidated informs the pendingBuilder that the height is is
// building the pending set has been fully validated and it can release any
// waiters waiting on the full set to be built.
func (pb *pendingBuilder) MarkHeightValidated() {
	close(pb.heightValidated)
}

// Close terminates the pending builder
func (pb *pendingBuilder) Close() {
	close(pb.done)
}

// mustHaveHeight ensures that the pendingBuilder is working on top of height,
// panics otherwise.
func (pb *pendingBuilder) mustHaveHeight(height *big.Int) {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	if pb.height.Cmp(height) != 0 {
		panic(fmt.Errorf("request for valid pending txs at height %d, but pending builder is at height %d", height, pb.height))
	}
}
