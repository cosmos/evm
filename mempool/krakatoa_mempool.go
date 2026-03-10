package mempool

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/evm/mempool/internal/heightsync"
	"github.com/cosmos/evm/mempool/internal/queue"
	"github.com/cosmos/evm/mempool/miner"
	"github.com/cosmos/evm/mempool/reserver"
	"github.com/cosmos/evm/mempool/txpool"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/rpc/stream"

	"cosmossdk.io/log/v2"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

var _ sdkmempool.ExtMempool = (*KrakatoaMempool)(nil)

// KrakatoaMempoolConfig extends the EVMMempoolConfig to have Krakatoa specific
// configuration options.
type KrakatoaMempoolConfig struct {
	EVMMempoolConfig
	// PendingTxProposalTimeout is the max amount of time to allocate to
	// fetching (or waiting to fetch) pending txs from the evm mempool.
	PendingTxProposalTimeout time.Duration
	// InsertQueueSize is how many txs can be stored in the insert queue
	// pending insertion into the mempool. Note the insert queue is only used
	// for EVM txs.
	InsertQueueSize int
}

// KrakatoaMempool is an application side mempool implementation that operates
// in conjunction with the CometBFT 'app' configuration. The KrakatoaMempool
// handles application side rechecking of txs and supports ABCI methods
// InesrtTx and ReapTxs.
type KrakatoaMempool struct {
	/** Keepers **/
	vmKeeper VMKeeperI

	/** Mempools **/
	txPool                   *txpool.TxPool
	legacyTxPool             *legacypool.LegacyPool
	recheckCosmosPool        *RecheckMempool
	operateExclusively       bool
	pendingTxProposalTimeout time.Duration

	/** Utils **/
	logger        log.Logger
	txConfig      client.TxConfig
	clientCtx     client.Context
	blockchain    *Blockchain
	blockGasLimit uint64 // Block gas limit from consensus parameters
	minTip        *uint256.Int

	eventBus *cmttypes.EventBus

	/** Transaction Reaping **/
	reapList *ReapList

	/** Transaction Tracking **/
	txTracker *txTracker

	/** Transaction Inserting **/
	cosmosInsertQueue *queue.Queue[sdk.Tx]
	evmInsertQueue    *queue.Queue[ethtypes.Transaction]
}

func NewKrakatoaMempool(
	getCtxCallback func(height int64, prove bool) (sdk.Context, error),
	logger log.Logger,
	vmKeeper VMKeeperI,
	feeMarketKeeper FeeMarketKeeperI,
	txConfig client.TxConfig,
	evmRechecker legacypool.Rechecker,
	cosmosRechecker Rechecker,
	config *KrakatoaMempoolConfig,
	cosmosPoolMaxTx int,
) *KrakatoaMempool {
	logger = logger.With(log.ModuleKey, "KrakatoaMempool")
	logger.Debug("creating new Krakatoa mempool")

	if config == nil {
		panic("config must not be nil")
	}
	if config.BlockGasLimit == 0 {
		logger.Warn("block gas limit is 0, setting to fallback", "fallback_limit", fallbackBlockGasLimit)
		config.BlockGasLimit = fallbackBlockGasLimit
	}
	blockchain := NewBlockchain(getCtxCallback, logger, vmKeeper, feeMarketKeeper, config.BlockGasLimit)

	legacyConfig := legacypool.DefaultConfig
	if config.LegacyPoolConfig != nil {
		legacyConfig = *config.LegacyPoolConfig
	}
	legacyPool := legacypool.New(legacyConfig, blockchain, legacypool.WithRecheck(evmRechecker))

	tracker := reserver.NewReservationTracker()
	txPool, err := txpool.New(uint64(0), blockchain, tracker, []txpool.SubPool{legacyPool})
	if err != nil {
		panic(err)
	}
	if len(txPool.Subpools) != 1 {
		panic("tx pool should contain one subpool")
	}
	if _, ok := txPool.Subpools[0].(*legacypool.LegacyPool); !ok {
		panic("tx pool should contain only legacypool")
	}

	cosmosPoolConfig := config.CosmosPoolConfig
	if cosmosPoolConfig == nil {
		// Default configuration
		defaultConfig := sdkmempool.PriorityNonceMempoolConfig[math.Int]{}
		defaultConfig.TxPriority = sdkmempool.TxPriority[math.Int]{
			GetTxPriority: func(goCtx context.Context, tx sdk.Tx) math.Int {
				ctx := sdk.UnwrapSDKContext(goCtx)
				cosmosTxFee, ok := tx.(sdk.FeeTx)
				if !ok {
					return math.ZeroInt()
				}
				found, coin := cosmosTxFee.GetFee().Find(vmKeeper.GetEvmCoinInfo(ctx).Denom)
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
		cosmosPoolConfig = &defaultConfig
	}
	cosmosPoolConfig.MaxTx = cosmosPoolMaxTx
	cosmosPool := sdkmempool.NewPriorityMempool(*cosmosPoolConfig)
	recheckPool := NewRecheckMempool(
		logger,
		cosmosPool,
		tracker.NewHandle(-1),
		cosmosRechecker,
		heightsync.New(blockchain.CurrentBlock().Number, NewCosmosTxStore),
		blockchain,
	)

	krakatoaMempool := &KrakatoaMempool{
		vmKeeper:                 vmKeeper,
		txPool:                   txPool,
		legacyTxPool:             txPool.Subpools[0].(*legacypool.LegacyPool),
		recheckCosmosPool:        recheckPool,
		logger:                   logger,
		txConfig:                 txConfig,
		blockchain:               blockchain,
		blockGasLimit:            config.BlockGasLimit,
		minTip:                   config.MinTip,
		operateExclusively:       true,
		pendingTxProposalTimeout: config.PendingTxProposalTimeout,
		reapList:                 NewReapList(NewTxEncoder(txConfig)),
		txTracker:                newTxTracker(),
	}

	// Setup queues
	krakatoaMempool.evmInsertQueue = queue.New(
		func(txs []*ethtypes.Transaction) []error {
			return txPool.Add(txs, false)
		},
		config.InsertQueueSize,
	)

	krakatoaMempool.cosmosInsertQueue = queue.New(
		func(txs []*sdk.Tx) []error {
			errs := make([]error, len(txs))
			for i, tx := range txs {
				// NOTE: cosmos txs must be added to the reap list directly
				// after insert, since recheck runs on insert, if insert
				// completes successfully, then we know they are valid and
				// should be added to the reap list, we do not need to wait
				// until the next blocks recheck.
				errs[i] = krakatoaMempool.insertAndReapCosmosTx(*tx)
			}
			return errs
		},
		config.InsertQueueSize,
	)

	legacyPool.OnTxEnqueued = krakatoaMempool.onEVMTxEnqueued()
	legacyPool.OnTxPromoted = krakatoaMempool.onEVMTxPromoted()
	legacyPool.OnTxRemoved = krakatoaMempool.onEVMTxRemoved()

	vmKeeper.SetEvmMempool(krakatoaMempool)

	// Start the cosmos pool recheck loop
	krakatoaMempool.recheckCosmosPool.Start(blockchain.CurrentBlock())

	return krakatoaMempool
}

// onEVMTxEnqueued defines a hook to run whenever an evm tx enters the queued pool.
func (m *KrakatoaMempool) onEVMTxEnqueued() func(tx *ethtypes.Transaction) {
	return func(tx *ethtypes.Transaction) {
		_ = m.txTracker.EnteredQueued(tx.Hash())
	}
}

// onEVMTxEnqueued defines a hook to run whenever an evm tx is promoted from
// the queued pool to the pending pool.
func (m *KrakatoaMempool) onEVMTxPromoted() func(tx *ethtypes.Transaction) {
	return func(tx *ethtypes.Transaction) {
		// once we have validated that the tx is valid (and can be promoted, set it
		// to be reaped)
		if err := m.reapList.PushEVMTx(tx); err != nil {
			m.logger.Error("could not push promoted evm tx to ReapList", "err", err)
		}

		hash := tx.Hash()
		_ = m.txTracker.ExitedQueued(hash)
		_ = m.txTracker.EnteredPending(hash)
	}
}

// onEVMTxEnqueued defines a hook to run whenever an evm tx is removed from a
// pool (queued or pending).
func (m *KrakatoaMempool) onEVMTxRemoved() func(tx *ethtypes.Transaction, pool legacypool.PoolType) {
	return func(tx *ethtypes.Transaction, pool legacypool.PoolType) {
		// tx was invalidated for some reason or was included in a block
		// (either way it is no longer in the mempool), if this tx is in the
		// reap list we need remove it from there (no longer need to gossip to
		// others about the tx) + the reap guard (since we may see this tx at a
		// later time, in which case we should gossip it again) by readding to
		// the reap guard.
		m.reapList.DropEVMTx(tx)

		_ = m.txTracker.RemoveTxFromPool(tx.Hash(), pool)
	}
}

// IsExclusive returns true if this mempool is the ONLY mempool in the chain.
func (m *KrakatoaMempool) IsExclusive() bool {
	return true
}

// GetBlockchain returns the blockchain interface used for chain head event notifications.
// This is primarily used to notify the mempool when new blocks are finalized.
func (m *KrakatoaMempool) GetBlockchain() *Blockchain {
	return m.blockchain
}

// GetTxPool returns the underlying EVM txpool.
// This provides direct access to the EVM-specific transaction management functionality.
func (m *KrakatoaMempool) GetTxPool() *txpool.TxPool {
	return m.txPool
}

// SetClientCtx sets the client context provider for broadcasting transactions
func (m *KrakatoaMempool) SetClientCtx(clientCtx client.Context) {
	m.clientCtx = clientCtx
}

// Insert adds a transaction to the appropriate mempool (EVM or Cosmos).
// EVM transactions are routed to the EVM transaction pool, while all other
// transactions are inserted into the Cosmos sdkmempool.
func (m *KrakatoaMempool) Insert(ctx context.Context, tx sdk.Tx) error {
	errC, err := m.insert(ctx, tx)
	if err != nil {
		return fmt.Errorf("inserting tx: %w", err)
	}

	if errC != nil {
		// if we got back a non nil async error channel, wait for that to
		// resolve
		select {
		case err := <-errC:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// InsertAsync adds a transaction to the appropriate mempool (EVM or Cosmos). EVM
// transactions are routed to the EVM transaction pool, while all other
// transactions are inserted into the Cosmos sdkmempool. EVM transactions are
// inserted async, i.e. they are scheduled for promotion only, we do not wait
// for it to complete.
func (m *KrakatoaMempool) InsertAsync(ctx context.Context, tx sdk.Tx) error {
	errC, err := m.insert(ctx, tx)
	if err != nil {
		return fmt.Errorf("inserting tx: %w", err)
	}

	select {
	case err := <-errC:
		// if we have a result immediately, ready on the channel returned from
		// insert, return that (cosmos tx or unable to try and insert the tx
		// due to parsing error).
		return err
	case <-ctx.Done():
		return ctx.Err()
	default:
		// result was not ready immediately, return nil while async things happen
		return nil
	}
}

// insert inserts a tx into its respective mempool, returning a channel for any
// async errors that may happen later upon actual mempool insertion, and an
// error for any errors that occurred synchronously.
func (m *KrakatoaMempool) insert(_ context.Context, tx sdk.Tx) (<-chan error, error) {
	ethMsg, err := evmTxFromCosmosTx(tx)
	switch {
	case err == nil:
		ethTx := ethMsg.AsTransaction()

		// we push the tx onto the evm insert queue so the tx will be inserted
		// at a later point. We get back a subscription that the insert queue
		// will use to notify the caller of any errors that occurred when
		// inserting into the mempool.
		return m.evmInsertQueue.Push(ethTx), nil
	case errors.Is(err, ErrMultiMsgEthereumTransaction):
		// there are multiple messages in this tx and one or more of them is an
		// evm tx, this is invalid
		return nil, err
	default:
		// we push the tx onto the cosmos insert queue so the tx will be
		// inserted at a later point. We get back a subscription that the
		// insert queue will use to notify the caller of any errors that
		// occurred when inserting into the mempool.
		return m.cosmosInsertQueue.Push(&tx), nil
	}
}

// insertAndReapCosmosTx inserts a cosmos tx into the cosmos mempool and sets
// it to be reaped.
func (m *KrakatoaMempool) insertAndReapCosmosTx(tx sdk.Tx) error {
	m.logger.Debug("inserting Cosmos transaction")

	// Insert into cosmos pool (handles locking, ante handler, and address reservation internally)
	if err := m.recheckCosmosPool.Insert(context.Background(), tx); err != nil {
		m.logger.Error("failed to insert Cosmos transaction", "error", err)
		return err
	}

	m.logger.Debug("Cosmos transaction inserted successfully")
	if err := m.reapList.PushCosmosTx(tx); err != nil {
		panic(fmt.Errorf("successfully inserted cosmos tx, but failed to insert into reap list: %w", err))
	}
	return nil
}

// ReapNewValidTxs removes and returns the oldest transactions from the reap
// list until maxBytes or maxGas limits are reached.
func (m *KrakatoaMempool) ReapNewValidTxs(maxBytes uint64, maxGas uint64) ([][]byte, error) {
	m.logger.Debug("reaping transactions", "maxBytes", maxBytes, "maxGas", maxGas, "available_txs")
	txs := m.reapList.Reap(maxBytes, maxGas)
	m.logger.Debug("reap complete", "txs_reaped", len(txs))

	return txs, nil
}

// Select returns a unified iterator over both EVM and Cosmos transactions.
// The iterator prioritizes transactions based on their fees and manages proper
// sequencing. The i parameter contains transaction hashes to exclude from selection.
func (m *KrakatoaMempool) Select(goCtx context.Context, i [][]byte) sdkmempool.Iterator {
	return m.buildIterator(goCtx, i)
}

// SelectBy iterates through transactions until the provided filter function returns false.
// It uses the same unified iterator as Select but allows early termination based on
// custom criteria defined by the filter function.
func (m *KrakatoaMempool) SelectBy(goCtx context.Context, txs [][]byte, filter func(sdk.Tx) bool) {
	defer func(t0 time.Time) { telemetry.MeasureSince(t0, "expmempool_selectby_duration") }(time.Now()) //nolint:staticcheck

	iter := m.buildIterator(goCtx, txs)

	for iter != nil && filter(iter.Tx()) {
		iter = iter.Next()
	}
}

// buildIterator ensures that EVM mempool has checked txs for reorgs up to COMMITTED
// block height and then returns a combined iterator over EVM & Cosmos txs.
func (m *KrakatoaMempool) buildIterator(ctx context.Context, txs [][]byte) sdkmempool.Iterator {
	defer func(t0 time.Time) { telemetry.MeasureSince(t0, "expmempool_builditerator_duration") }(time.Now()) //nolint:staticcheck

	evmIterator, cosmosIterator := m.getIterators(ctx, txs)

	return NewEVMMempoolIterator(
		evmIterator,
		cosmosIterator,
		m.logger,
		m.txConfig,
		m.vmKeeper.GetEvmCoinInfo(sdk.UnwrapSDKContext(ctx)).Denom,
		m.blockchain,
	)
}

// CountTx returns the total number of transactions in both EVM and Cosmos pools.
// This provides a combined count across all mempool types.
func (m *KrakatoaMempool) CountTx() int {
	pending, _ := m.txPool.Stats()
	return m.recheckCosmosPool.CountTx() + pending
}

// Remove fallbacks for RemoveWithReason
func (m *KrakatoaMempool) Remove(tx sdk.Tx) error {
	return m.RemoveWithReason(context.Background(), tx, sdkmempool.RemoveReason{
		Caller: "remove",
		Error:  nil,
	})
}

// RemoveWithReason removes a transaction from the appropriate sdkmempool.
// For EVM transactions, removal is typically handled automatically by the pool
// based on nonce progression. Cosmos transactions are removed from the Cosmos pool.
func (m *KrakatoaMempool) RemoveWithReason(ctx context.Context, tx sdk.Tx, reason sdkmempool.RemoveReason) error {
	chainCtx, err := m.blockchain.GetLatestContext()
	if err != nil || chainCtx.BlockHeight() == 0 {
		m.logger.Warn("Failed to get latest context, skipping removal")
		return nil
	}

	msgEthereumTx, err := evmTxFromCosmosTx(tx)
	switch {
	case errors.Is(err, ErrNoMessages):
		return err
	case err != nil:
		// unable to parse evm tx -> process as cosmos tx
		return m.removeCosmosTx(ctx, tx, reason)
	}

	hash := msgEthereumTx.Hash()

	if m.shouldRemoveFromEVMPool(hash, reason) {
		m.logger.Debug("Manually removing EVM transaction", "tx_hash", hash)
		m.legacyTxPool.RemoveTx(hash, false, true, convertRemovalReason(reason.Caller))
	}

	if reason.Caller == sdkmempool.CallerRunTxFinalize {
		_ = m.txTracker.IncludedInBlock(hash)
	}

	return nil
}

// removeCosmosTx removes a cosmos tx from the mempool.
// The RecheckMempool handles locking internally.
func (m *KrakatoaMempool) removeCosmosTx(ctx context.Context, tx sdk.Tx, reason sdkmempool.RemoveReason) error {
	m.logger.Debug("Removing Cosmos transaction")

	// Remove from cosmos pool (handles address reservation release internally)
	err := sdkmempool.RemoveWithReason(ctx, m.recheckCosmosPool, tx, reason)
	if err != nil {
		m.logger.Error("Failed to remove Cosmos transaction", "error", err)
		return err
	}

	m.reapList.DropCosmosTx(tx)
	m.logger.Debug("Cosmos transaction removed successfully")

	return nil
}

// shouldRemoveFromEVMPool determines whether an EVM transaction should be manually removed.
func (m *KrakatoaMempool) shouldRemoveFromEVMPool(hash common.Hash, reason sdkmempool.RemoveReason) bool {
	if reason.Error == nil {
		return false
	}
	// Comet will attempt to remove transactions from the mempool after completing successfully.
	// We should not do this with EVM transactions because removing them causes the subsequent ones to
	// be dequeued as temporarily invalid, only to be requeued a block later.
	// The EVM mempool handles removal based on account nonce automatically.
	isKnown := errors.Is(reason.Error, ErrNonceGap) ||
		errors.Is(reason.Error, sdkerrors.ErrInvalidSequence) ||
		errors.Is(reason.Error, sdkerrors.ErrOutOfGas)

	if isKnown {
		m.logger.Debug("Transaction validation succeeded, should be kept", "tx_hash", hash, "caller", reason.Caller)
		return false
	}

	m.logger.Debug("Transaction validation failed, should be removed", "tx_hash", hash, "caller", reason.Caller)
	return true
}

// SetEventBus sets CometBFT event bus to listen for new block header event.
func (m *KrakatoaMempool) SetEventBus(eventBus *cmttypes.EventBus) {
	if m.HasEventBus() {
		m.eventBus.Unsubscribe(context.Background(), SubscriberName, stream.NewBlockHeaderEvents) //nolint: errcheck
	}
	m.eventBus = eventBus
	sub, err := eventBus.Subscribe(context.Background(), SubscriberName, stream.NewBlockHeaderEvents)
	if err != nil {
		panic(err)
	}
	go func() {
		bc := m.GetBlockchain()
		for range sub.Out() {
			bc.NotifyNewBlock()
			// Trigger cosmos pool recheck on new block (non-blocking)
			m.recheckCosmosPool.TriggerRecheck(bc.CurrentBlock())
		}
	}()
}

// HasEventBus returns true if the blockchain is configured to use an event bus for block notifications.
func (m *KrakatoaMempool) HasEventBus() bool {
	return m.eventBus != nil
}

func (m *KrakatoaMempool) Close() error {
	var errs []error
	if m.eventBus != nil {
		if err := m.eventBus.Unsubscribe(context.Background(), SubscriberName, stream.NewBlockHeaderEvents); err != nil {
			errs = append(errs, fmt.Errorf("failed to unsubscribe from event bus: %w", err))
		}
	}

	if err := m.recheckCosmosPool.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close cosmos pool: %w", err))
	}

	if err := m.txPool.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close txpool: %w", err))
	}

	return errors.Join(errs...)
}

// getIterators prepares iterators over pending EVM and Cosmos transactions.
// It configures EVM transactions with proper base fee filtering and priority ordering,
// while setting up the Cosmos iterator with the provided exclusion list.
func (m *KrakatoaMempool) getIterators(ctx context.Context, _ [][]byte) (evm *miner.TransactionsByPriceAndNonce, cosmos sdkmempool.Iterator) {
	var (
		evmIterator    *miner.TransactionsByPriceAndNonce
		cosmosIterator sdkmempool.Iterator
		wg             sync.WaitGroup
	)

	// using ctx.BlockHeight() - 1 since we want to get txs that have been
	// validated at latest committed height, and ctx.BlockHeight() returns the
	// latest uncommitted height
	selectHeight := new(big.Int).SetInt64(sdk.UnwrapSDKContext(ctx).BlockHeight() - 1)

	wg.Go(func() {
		evmIterator = m.evmIterator(ctx, selectHeight)
	})

	wg.Go(func() {
		cosmosIterator = m.cosmosIterator(ctx, selectHeight)
	})

	wg.Wait()

	return evmIterator, cosmosIterator
}

// evmIterator returns an iterator over the current valid txs in the evm
// mempool at height.
func (m *KrakatoaMempool) evmIterator(ctx context.Context, height *big.Int) *miner.TransactionsByPriceAndNonce {
	sdkctx := sdk.UnwrapSDKContext(ctx)
	baseFee := m.vmKeeper.GetBaseFee(sdkctx)
	var baseFeeUint *uint256.Int
	if baseFee != nil {
		baseFeeUint = uint256.MustFromBig(baseFee)
	}

	filter := txpool.PendingFilter{
		MinTip:       m.minTip,
		BaseFee:      baseFeeUint,
		BlobFee:      nil,
		OnlyPlainTxs: true,
		OnlyBlobTxs:  false,
	}

	if m.pendingTxProposalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.pendingTxProposalTimeout)
		defer cancel()
	}
	evmPendingTxs := m.txPool.Rechecked(ctx, height, filter)
	return miner.NewTransactionsByPriceAndNonce(nil, evmPendingTxs, baseFee)
}

// cosmosIterator returns an iterator over the current valid txs in the cosmos
// mempool at height.
func (m *KrakatoaMempool) cosmosIterator(ctx context.Context, height *big.Int) sdkmempool.Iterator {
	if m.pendingTxProposalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.pendingTxProposalTimeout)
		defer cancel()
	}
	return m.recheckCosmosPool.RecheckedTxs(ctx, height)
}

// TrackTx submits a tx to be tracked for its tx inclusion metrics.
func (m *KrakatoaMempool) TrackTx(hash common.Hash) error {
	return m.txTracker.Track(hash)
}

// RecheckCosmosTxs triggers a synchronous recheck of cosmos transactions.
// This is primarily used for testing.
func (m *KrakatoaMempool) RecheckCosmosTxs(newHead *ethtypes.Header) {
	m.recheckCosmosPool.TriggerRecheckSync(newHead)
}
