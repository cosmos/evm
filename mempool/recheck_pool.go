package mempool

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"go.opentelemetry.io/otel/metric"

	"github.com/cosmos/evm/mempool/internal/heightsync"
	"github.com/cosmos/evm/mempool/internal/reaplist"
	"github.com/cosmos/evm/mempool/reserver"

	"cosmossdk.io/log/v2"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
)

var (
	recheckDuration   metric.Float64Histogram
	recheckRemovals   metric.Int64Histogram
	recheckNumChecked metric.Int64Histogram
)

func init() {
	var err error
	recheckDuration, err = meter.Float64Histogram(
		"mempool.recheck.duration",
		metric.WithDescription("Duration of cosmos mempool recheck loop"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		panic(err)
	}

	recheckRemovals, err = meter.Int64Histogram(
		"mempool.recheck.removals",
		metric.WithDescription("Number of transactions that were removed from the pool per iteration"),
	)
	if err != nil {
		panic(err)
	}

	recheckNumChecked, err = meter.Int64Histogram(
		"mempool.recheck.num_checked",
		metric.WithDescription("Number of transactions rechecked per iteration"),
	)
	if err != nil {
		panic(err)
	}
}

// Rechecker defines the minimal set of methods needed to recheck cosmos
// transactions and manage the context that the transactions are rechecked
// against.
type Rechecker interface {
	// GetContext gets a branch of the current context that transactions should
	// be rechecked against. Changes to ctx will only be persisted back to the
	// Rechecker once the write function is invoked.
	GetContext() (ctx sdk.Context, write func())

	// RecheckCosmos performs validation of a cosmos tx against a context, and
	// returns an updated context.
	RecheckCosmos(ctx sdk.Context, tx sdk.Tx) (sdk.Context, error)

	// Update updates the recheckers context to be the ctx at headers height.
	Update(ctx sdk.Context, header *ethtypes.Header)
}

// LatestContextProvider provides the minimal methods needed by RecheckMempool
// for context management during rechecks.
type LatestContextProvider interface {
	GetLatestContext() (sdk.Context, error)
	CurrentBlock() *ethtypes.Header
}

// RecheckMempool wraps an ExtMempool and provides block-driven rechecking
// of transactions when new blocks are committed. It mirrors the legacypool
// pattern but simplified for Cosmos mempool behavior (no reorgs, no queued/pending management).
//
// All pool mutations (Insert, Remove) and reads (Select, CountTx) are protected
// by a RWMutex to ensure thread-safety during recheck operations.
type RecheckMempool struct {
	sdkmempool.ExtMempool

	// mu protects the pool during mutations and reads.
	// Write lock: Insert, Remove, runRecheck
	// Read lock: Select, CountTx
	mu sync.RWMutex

	// reserver coordinates address reservations with other pools (i.e. legacypool)
	reserver *reserver.ReservationHandle

	rechecker  Rechecker
	blockchain *Blockchain
	logger     log.Logger

	// event channels
	reqRecheckCh    chan *recheckRequest // channel that schedules rechecking.
	recheckDoneCh   chan chan struct{}   // channel that is returned to recheck callers that signals when a recheck is complete.
	shutdownCh      chan struct{}        // shutdown channel to gracefully shutdown the recheck loop.
	shutdownOnce    sync.Once            // ensures shutdown channel is only closed once.
	recheckShutdown chan struct{}        // closed when scheduleRecheckLoop exits

	// recheckedTxs is a height synced CosmosTxStore, used to collect txs that
	// have been rechecked at a height, and discard of them once the chain.
	recheckedTxs *heightsync.HeightSync[CosmosTxStore]

	reapList *reaplist.ReapList

	wg sync.WaitGroup
}

// NewRecheckMempool creates a new RecheckMempool wrapping the given pool.
func NewRecheckMempool(
	logger log.Logger,
	cosmosPoolConfig *sdkmempool.PriorityNonceMempoolConfig[math.Int],
	maxTxs int,
	reserver *reserver.ReservationHandle,
	rechecker Rechecker,
	recheckedTxs *heightsync.HeightSync[CosmosTxStore],
	reapList *reaplist.ReapList,
	blockchain *Blockchain,
) *RecheckMempool {
	if cosmosPoolConfig == nil {
		cosmosPoolConfig = defaultCosmosPoolConfig(reapList, blockchain)
	}
	cosmosPoolConfig.MaxTx = maxTxs

	return &RecheckMempool{
		ExtMempool:      sdkmempool.NewPriorityMempool(*cosmosPoolConfig),
		reserver:        reserver,
		rechecker:       rechecker,
		blockchain:      blockchain,
		logger:          logger.With(log.ModuleKey, "RecheckMempool"),
		reqRecheckCh:    make(chan *recheckRequest),
		recheckDoneCh:   make(chan chan struct{}),
		shutdownCh:      make(chan struct{}),
		recheckShutdown: make(chan struct{}),
		reapList:        reapList,
		recheckedTxs:    recheckedTxs,
	}
}

// Start begins the background recheck loop and initializes the rechecker's
// context to the latest chain state. The initialHead is used for the first
// Rechecker.Update call before any recheck has been triggered.
func (m *RecheckMempool) Start(initialHead *ethtypes.Header) {
	ctx, err := m.blockchain.GetLatestContext()
	if err != nil {
		m.logger.Error("failed to initialize rechecker context", "err", err)
	} else {
		m.rechecker.Update(ctx, initialHead)
	}

	m.wg.Add(1)
	go m.scheduleRecheckLoop()
}

// Close gracefully shuts down the recheck loop.
func (m *RecheckMempool) Close() error {
	m.shutdownOnce.Do(func() {
		close(m.shutdownCh)
	})
	m.wg.Wait()
	return nil
}

// Insert adds a transaction to the pool after running the ante handler.
// This is the main entry point for new cosmos transactions.
func (m *RecheckMempool) Insert(_ context.Context, tx sdk.Tx) error {
	// Reserve addresses to prevent conflicts with EVM pool
	addrs, err := signerAddressesFromTx(tx)
	if err != nil {
		return err
	}
	if err := m.reserver.Hold(addrs...); err != nil {
		return fmt.Errorf("reserving %d addresses for cosmos recheck pool: %w", len(addrs), err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Branch from the Rechecker's internal ctx (post-recheck cache).
	// This ctx has chain_state + all pending txs' nonce increments.
	ctx, write := m.rechecker.GetContext()
	if ctx.IsZero() {
		m.logger.Warn("no context found in rechecker on insert, updating to latest")
		// if this happens, we have not rechecked any txs yet, so this is safe
		// to update
		newCtx, err := m.blockchain.GetLatestContext()
		if err != nil {
			return fmt.Errorf("fetching latest context since rechecker has none: %w", err)
		}

		m.rechecker.Update(newCtx, m.blockchain.CurrentBlock())
		ctx, write = m.rechecker.GetContext()
	}

	if _, err := m.rechecker.RecheckCosmos(ctx, tx); err != nil {
		_ = m.reserver.Release(addrs...) // best effort cleanup
		return fmt.Errorf("ante handler failed: %w", err)
	}

	if err := m.ExtMempool.Insert(ctx, tx); err != nil {
		_ = m.reserver.Release(addrs...) // best effort cleanup
		return err
	}

	// since we have rechecked the tx via `rechecker.RecheckCosmos`, and this
	// rechecks the tx on top of the state of all txs already rechecked in the
	// mempool, this tx is valid and we can include it in the reaplist
	if err := m.reapList.PushCosmosTx(tx); err != nil {
		m.logger.Error("successfully inserted cosmos tx, but failed to insert into reap list", "err", err)
	}

	write()
	m.markTxInserted(tx)
	return nil
}

// Remove removes a transaction from the pool.
func (m *RecheckMempool) Remove(tx sdk.Tx) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.removeLocked(tx)
}

// RemoveWithReason removes a transaction from the pool. This must be
// explicitly defined to prevent Go from promoting the embedded ExtMempool's
// RemoveWithReason, which would bypass the reserver release logic.
func (m *RecheckMempool) RemoveWithReason(_ context.Context, tx sdk.Tx, _ sdkmempool.RemoveReason) error {
	return m.Remove(tx)
}

// removeLocked removes a tx from the underlying pool and releases the
// reserver. Caller must hold m.mu.
func (m *RecheckMempool) removeLocked(tx sdk.Tx) error {
	if err := m.ExtMempool.Remove(tx); err != nil {
		return err
	}

	addrs, err := signerAddressesFromTx(tx)
	if err != nil {
		panic("failed to extract signer addresses from tx during Remove")
	}
	m.reserver.Release(addrs...) //nolint:errcheck // best effort cleanup
	m.reapList.DropCosmosTx(tx)

	return nil
}

// Select returns an iterator over transactions in the pool.
func (m *RecheckMempool) Select(ctx context.Context, txs [][]byte) sdkmempool.Iterator {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ExtMempool.Select(ctx, txs)
}

// CountTx returns the number of transactions in the pool.
func (m *RecheckMempool) CountTx() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ExtMempool.CountTx()
}

type recheckRequest struct {
	newHead *ethtypes.Header
}

// TriggerRecheck signals that a new block arrived and returns a channel
// that closes when the recheck completes (or is superseded by another).
func (m *RecheckMempool) TriggerRecheck(newHead *ethtypes.Header) <-chan struct{} {
	select {
	case m.reqRecheckCh <- &recheckRequest{newHead: newHead}:
		return <-m.recheckDoneCh
	case <-m.recheckShutdown:
		ch := make(chan struct{})
		close(ch)
		return ch
	}
}

// TriggerRecheckSync triggers a recheck and blocks until complete.
func (m *RecheckMempool) TriggerRecheckSync(newHead *ethtypes.Header) {
	<-m.TriggerRecheck(newHead)
}

// RecheckedTxs returns the txs that have been rechecked for a height. The
// RecheckMempool must be currently operating on this height (i.e. recheck has
// been triggered on this height via TriggerRecheck). If height is in the past
// (TriggerRecheck has been called on height + 1), this will panic. If height
// is in the future, this will block until TriggerReset is called for height,
// or the context times out.
func (m *RecheckMempool) RecheckedTxs(ctx context.Context, height *big.Int) sdkmempool.Iterator {
	txStore := m.recheckedTxs.GetStore(ctx, height)
	if txStore == nil {
		return nil
	}
	return txStore.Iterator()
}

// OrderedRecheckedTxs returns the rechecked tx snapshot for a height using
// fee-priority ordering across signer buckets while still honoring nonce order
// within each bucket.
func (m *RecheckMempool) OrderedRecheckedTxs(
	ctx context.Context,
	height *big.Int,
	bondDenom string,
	baseFee *uint256.Int,
) sdkmempool.Iterator {
	txStore := m.recheckedTxs.GetStore(ctx, height)
	if txStore == nil {
		return nil
	}
	return txStore.OrderedIterator(bondDenom, baseFee)
}

// scheduleRecheckLoop is the main event loop that coordinates recheck execution.
// Only one recheck runs at a time. If a new block arrives while a recheck is
// running, the current recheck is cancelled and a new one is scheduled.
func (m *RecheckMempool) scheduleRecheckLoop() {
	defer m.wg.Done()
	defer close(m.recheckShutdown)

	var (
		curDone       chan struct{} // non-nil while recheck is running
		nextDone      = make(chan struct{})
		launchNextRun bool
		recheckReq    *recheckRequest
		cancelCh      chan struct{} // closed to signal cancellation
	)

	for {
		if curDone == nil && launchNextRun {
			cancelCh = make(chan struct{})
			go m.runRecheck(nextDone, recheckReq.newHead, cancelCh)

			curDone, nextDone = nextDone, make(chan struct{})
			launchNextRun = false
			recheckReq = nil
		}

		select {
		case req := <-m.reqRecheckCh:
			if curDone != nil && cancelCh != nil {
				close(cancelCh)
				cancelCh = nil
			}
			recheckReq = req
			launchNextRun = true
			m.recheckDoneCh <- nextDone

		case <-curDone:
			curDone = nil
			cancelCh = nil

		case <-m.shutdownCh:
			if curDone != nil {
				if cancelCh != nil {
					close(cancelCh)
				}
				<-curDone
			}
			close(nextDone)
			return
		}
	}
}

// runRecheck performs the actual recheck work. It holds the write lock for the
// entire duration, iterates through all txs, runs them through the ante handler,
// and removes any that fail (plus dependent txs with higher sequences).
func (m *RecheckMempool) runRecheck(done chan struct{}, newHead *ethtypes.Header, cancelled <-chan struct{}) {
	defer close(done)
	start := time.Now()
	txsRemoved := 0
	txsChecked := 0
	defer func() {
		recheckDuration.Record(context.Background(), float64(time.Since(start).Milliseconds()))
		if txsRemoved > 0 {
			recheckRemovals.Record(context.Background(), int64(txsRemoved))
		}
		if txsChecked > 0 {
			recheckNumChecked.Record(context.Background(), int64(txsChecked))
		}
	}()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.recheckedTxs.StartNewHeight(newHead.Number)
	defer m.recheckedTxs.EndCurrentHeight()

	latestCtx, err := m.blockchain.GetLatestContext()
	if err != nil {
		m.logger.Error("failed to get context for recheck", "err", err)
		return
	}
	m.rechecker.Update(latestCtx, newHead)

	failedAtSequence := make(map[string]uint64)
	removeTxs := make([]sdk.Tx, 0)

	// Branch from the rechecker for iteration context. Each successful tx's
	// state changes are committed back to the Rechecker immediately via
	// write()
	ctx, write := m.rechecker.GetContext()

	iter := m.ExtMempool.Select(ctx, nil)
	for iter != nil {
		if isCancelled(cancelled) {
			m.logger.Debug("recheck cancelled - new block arrived")
			return
		}

		txn := iter.Tx()
		if txn == nil {
			break
		}

		txsChecked++
		signerSeqs, err := extractSignerSequences(txn)
		if err != nil {
			m.logger.Error("failed to extract signer sequences", "err", err)
			iter = iter.Next()
			continue
		}

		invalidTx := false
		for _, sig := range signerSeqs {
			if failedSeq, ok := failedAtSequence[sig.account]; ok && failedSeq < sig.seq {
				invalidTx = true
				break
			}
		}

		if !invalidTx {
			if _, err := m.rechecker.RecheckCosmos(ctx, txn); err == nil {
				write()
				m.markTxRechecked(txn)
				iter = iter.Next()
				ctx, write = m.rechecker.GetContext()
				continue
			}
		}

		removeTxs = append(removeTxs, txn)
		for _, sig := range signerSeqs {
			if existing, ok := failedAtSequence[sig.account]; !ok || existing > sig.seq {
				failedAtSequence[sig.account] = sig.seq
			}
		}

		iter = iter.Next()
	}

	if isCancelled(cancelled) {
		m.logger.Debug("recheck cancelled before removal - new block arrived")
		return
	}

	for _, txn := range removeTxs {
		if err := m.ExtMempool.Remove(txn); err != nil {
			m.logger.Error("failed to remove tx during recheck", "err", err)
			continue
		}
		m.reapList.DropCosmosTx(txn)

		addrs, err := signerAddressesFromTx(txn)
		if err != nil {
			m.logger.Error("failed to extract signer addresses for release", "err", err)
			continue
		}
		m.reserver.Release(addrs...) //nolint:errcheck // best effort cleanup
	}
	txsRemoved = len(removeTxs)
}

// markTxRechecked adds a tx into the height synced cosmos tx store.
func (m *RecheckMempool) markTxRechecked(txn sdk.Tx) {
	m.recheckedTxs.Do(func(store *CosmosTxStore) { store.AddTx(txn) })
}

// markTxInserted conservatively updates the current height snapshot for live inserts.
// If the inserted tx replaces an existing tx, any other txs from the same sender with
// a higher nonce is dropped and rebuilt by the next recheck.
func (m *RecheckMempool) markTxInserted(txn sdk.Tx) {
	m.recheckedTxs.Do(func(store *CosmosTxStore) {
		if store.InvalidateFrom(txn) > 0 {
			return
		}
		store.AddTx(txn)
	})
}

type signerSequence struct {
	account string
	seq     uint64
}

// extractSignerSequences extracts account addresses and sequences from a tx.
func extractSignerSequences(txn sdk.Tx) ([]signerSequence, error) {
	sigTx, ok := txn.(authsigning.SigVerifiableTx)
	if !ok {
		return nil, fmt.Errorf(
			"tx does not implement %T",
			(*authsigning.SigVerifiableTx)(nil),
		)
	}

	sigs, err := sigTx.GetSignaturesV2()
	if err != nil {
		return nil, err
	}

	signerSeqs := make([]signerSequence, 0, len(sigs))
	for _, sig := range sigs {
		signerSeqs = append(signerSeqs, signerSequence{
			account: sig.PubKey.Address().String(),
			seq:     sig.Sequence,
		})
	}

	return signerSeqs, nil
}

// isCancelled checks if the cancellation channel has been closed.
func isCancelled(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// signerAddressesFromTx extracts signer addresses from a transaction as EVM addresses.
func signerAddressesFromTx(tx sdk.Tx) ([]common.Address, error) {
	sigTx, ok := tx.(authsigning.SigVerifiableTx)
	if !ok {
		return nil, fmt.Errorf("tx does not implement GetSigners")
	}

	signers, err := sigTx.GetSigners()
	if err != nil {
		return nil, err
	}

	if len(signers) == 0 {
		return nil, fmt.Errorf("tx contains no signers")
	}

	addrs := make([]common.Address, 0, len(signers))
	for _, signer := range signers {
		addrs = append(addrs, common.BytesToAddress(signer))
	}

	return addrs, nil
}

func defaultCosmosPoolConfig(reapList *reaplist.ReapList, blockchain *Blockchain) *sdkmempool.PriorityNonceMempoolConfig[math.Int] {
	txPriority := sdkmempool.TxPriority[math.Int]{
		GetTxPriority: func(_ context.Context, tx sdk.Tx) math.Int {
			cosmosTxFee, ok := tx.(sdk.FeeTx)
			if !ok {
				return math.ZeroInt()
			}
			found, coin := cosmosTxFee.GetFee().Find(blockchain.GetCoinDenom())
			if !found {
				return math.ZeroInt()
			}

			gasPrice := coin.Amount.Quo(math.NewIntFromUint64(cosmosTxFee.GetGas()))

			return gasPrice
		},
		Compare: func(a, b math.Int) int {
			return a.BigInt().Cmp(b.BigInt())
		},
		MinValue: math.ZeroInt(),
	}

	txReplacement := func(oldPriority, newPriority math.Int, oldTx, newTx sdk.Tx) bool {
		// tx is being replaced, we need to drop the tx that is going to be removed
		// from the reap list. we assume that the tx doing the replacing has
		// already been inserted into the reaplist via the insert.
		reapList.DropCosmosTx(oldTx)
		return true
	}

	return &sdkmempool.PriorityNonceMempoolConfig[math.Int]{
		TxPriority:    txPriority,
		TxReplacement: txReplacement,
	}
}
