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
	"go.opentelemetry.io/otel"

	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/evm/mempool/internal/heightsync"
	"github.com/cosmos/evm/mempool/internal/queue"
	"github.com/cosmos/evm/mempool/internal/reaplist"
	"github.com/cosmos/evm/mempool/internal/txtracker"
	"github.com/cosmos/evm/mempool/miner"
	"github.com/cosmos/evm/mempool/reserver"
	"github.com/cosmos/evm/mempool/txpool"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/rpc/stream"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log/v2"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

const (
	// SubscriberName is the name of the event bus subscriber for the EVM mempool
	SubscriberName = "evm"
	// fallbackBlockGasLimit is the default block gas limit is 0 or missing in genesis file
	fallbackBlockGasLimit = 100_000_000
)

// AllowUnsafeSyncInsert indicates whether to perform synchronous inserts into the mempool
// for testing purposes. When true, Insert will block until the transaction is fully processed.
// This should be used only in tests to ensure deterministic behavior
var AllowUnsafeSyncInsert = false

var meter = otel.Meter("github.com/cosmos/evm/mempool")

var _ sdkmempool.ExtMempool = (*Mempool)(nil)

// Config defines the configuration for the EVM mempool.
type Config struct {
	LegacyPoolConfig *legacypool.Config
	CosmosPoolConfig *sdkmempool.PriorityNonceMempoolConfig[math.Int]
	AnteHandler      sdk.AnteHandler
	BroadCastTxFn    func(txs []*ethtypes.Transaction) error

	// Block gas limit from consensus parameters
	BlockGasLimit uint64
	MinTip        *uint256.Int

	// PendingTxProposalTimeout is the max amount of time to allocate to
	// fetching (or waiting to fetch) pending txs from the evm mempool.
	PendingTxProposalTimeout time.Duration
	// InsertQueueSize is how many txs can be stored in the insert queue
	// pending insertion into the mempool. Note the insert queue is only used
	// for EVM txs.
	InsertQueueSize int
}

// Mempool is an application side mempool implementation that operates
// in conjunction with the CometBFT 'app' configuration. The Mempool
// handles application side rechecking of txs and supports ABCI methods
// InesrtTx and ReapTxs.
type Mempool struct {
	/** Keepers **/
	vmKeeper VMKeeperI

	/** Mempools **/
	txPool                   *txpool.TxPool
	legacyTxPool             *legacypool.LegacyPool
	recheckCosmosPool        *RecheckMempool
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
	reapList *reaplist.ReapList

	/** Transaction Tracking **/
	txTracker *txtracker.TxTracker

	/** Transaction Inserting **/
	cosmosInsertQueue *queue.Queue[sdk.Tx]
	evmInsertQueue    *queue.Queue[ethtypes.Transaction]

	/** Signer extraction **/
	signerExtractor sdkmempool.SignerExtractionAdapter
}

func NewMempool(
	getCtxCallback func(height int64, prove bool) (sdk.Context, error),
	logger log.Logger,
	vmKeeper VMKeeperI,
	feeMarketKeeper FeeMarketKeeperI,
	txConfig client.TxConfig,
	evmRechecker legacypool.Rechecker,
	cosmosRechecker Rechecker,
	config *Config,
	cosmosPoolMaxTx int,
) *Mempool {
	logger = logger.With(log.ModuleKey, "Mempool")
	logger.Debug("creating new mempool")

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

	reapList := reaplist.New(NewTxEncoder(txConfig))
	txTracker := txtracker.New()
	legacyPool := legacypool.New(
		legacyConfig,
		logger,
		blockchain,
		reapList,
		txTracker,
		legacypool.WithRecheck(evmRechecker),
	)

	reservationTracker := reserver.NewReservationTracker()
	txPool, err := txpool.New(uint64(0), blockchain, reservationTracker, []txpool.SubPool{legacyPool})
	if err != nil {
		panic(err)
	}
	if len(txPool.Subpools) != 1 {
		panic("tx pool should contain one subpool")
	}
	if _, ok := txPool.Subpools[0].(*legacypool.LegacyPool); !ok {
		panic("tx pool should contain only legacypool")
	}

	recheckPool := NewRecheckMempool(
		logger,
		config.CosmosPoolConfig,
		cosmosPoolMaxTx,
		reservationTracker.NewHandle(-1),
		cosmosRechecker,
		heightsync.New(blockchain.CurrentBlock().Number, NewCosmosTxStore, logger.With("pool", "cosmos_recheck_mempool")),
		reapList,
		blockchain,
	)

	mempool := &Mempool{
		vmKeeper:                 vmKeeper,
		txPool:                   txPool,
		legacyTxPool:             txPool.Subpools[0].(*legacypool.LegacyPool),
		recheckCosmosPool:        recheckPool,
		logger:                   logger,
		txConfig:                 txConfig,
		blockchain:               blockchain,
		blockGasLimit:            config.BlockGasLimit,
		minTip:                   config.MinTip,
		pendingTxProposalTimeout: config.PendingTxProposalTimeout,
		reapList:                 reapList,
		txTracker:                txTracker,
		signerExtractor:          NewEthSignerExtractionAdapter(sdkmempool.NewDefaultSignerExtractionAdapter()),
	}

	// Setup queues
	mempool.evmInsertQueue = queue.New(
		func(txs []*ethtypes.Transaction) []error {
			return txPool.Add(txs, AllowUnsafeSyncInsert)
		},
		config.InsertQueueSize,
	)

	mempool.cosmosInsertQueue = queue.New(
		func(txs []*sdk.Tx) []error {
			errs := make([]error, len(txs))
			for i, tx := range txs {
				if tx == nil {
					continue
				}
				errs[i] = recheckPool.Insert(context.Background(), *tx)
			}
			return errs
		},
		config.InsertQueueSize,
	)

	vmKeeper.SetEvmMempool(mempool)

	// Start the cosmos pool recheck loop
	mempool.recheckCosmosPool.Start(blockchain.CurrentBlock())

	return mempool
}

// GetBlockchain returns the blockchain interface used for chain head event notifications.
// This is primarily used to notify the mempool when new blocks are finalized.
func (m *Mempool) GetBlockchain() *Blockchain {
	return m.blockchain
}

// GetTxPool returns the underlying EVM txpool.
// This provides direct access to the EVM-specific transaction management functionality.
func (m *Mempool) GetTxPool() *txpool.TxPool {
	return m.txPool
}

// SetClientCtx sets the client context provider for broadcasting transactions
func (m *Mempool) SetClientCtx(clientCtx client.Context) {
	m.clientCtx = clientCtx
}

// Insert adds a transaction to the appropriate mempool (EVM or Cosmos).
// EVM transactions are routed to the EVM transaction pool, while all other
// transactions are inserted into the Cosmos sdkmempool.
func (m *Mempool) Insert(ctx context.Context, tx sdk.Tx) error {
	errC, err := m.insert(tx)
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
func (m *Mempool) InsertAsync(tx sdk.Tx) error {
	errChan, err := m.insert(tx)
	if err != nil {
		return fmt.Errorf("inserting tx: %w", err)
	}

	select {
	case err := <-errChan:
		// if we have a result immediately, ready on the channel returned from insert,
		// return that (cosmos tx or unable to try and insert the tx due to parsing error).
		return err
	default:
		// result was not ready immediately, return nil while async things happen
		return nil
	}
}

// insert inserts a tx into its respective mempool, returning a channel for any
// async errors that may happen later upon actual mempool insertion, and an
// error for any errors that occurred synchronously.
func (m *Mempool) insert(tx sdk.Tx) (<-chan error, error) {
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

// ReapNewValidTxs removes and returns the oldest transactions from the reap
// list until maxBytes or maxGas limits are reached.
func (m *Mempool) ReapNewValidTxs(maxBytes uint64, maxGas uint64) ([][]byte, error) {
	m.logger.Debug("reaping transactions", "maxBytes", maxBytes, "maxGas", maxGas, "available_txs")
	txs := m.reapList.Reap(maxBytes, maxGas)
	m.logger.Debug("reap complete", "txs_reaped", len(txs))

	return txs, nil
}

// Select returns a unified iterator over both EVM and Cosmos transactions.
// The iterator prioritizes transactions based on their fees and manages proper
// sequencing. The i parameter contains transaction hashes to exclude from selection.
func (m *Mempool) Select(goCtx context.Context, i [][]byte) sdkmempool.Iterator {
	return m.buildIterator(goCtx, i)
}

// SelectBy iterates through transactions until the provided filter function returns false.
// It uses the same unified iterator as Select but allows early termination based on
// custom criteria defined by the filter function.
func (m *Mempool) SelectBy(goCtx context.Context, txs [][]byte, filter func(sdk.Tx) bool) {
	defer func(t0 time.Time) { telemetry.MeasureSince(t0, "expmempool_selectby_duration") }(time.Now()) //nolint:staticcheck

	iter := m.buildIterator(goCtx, txs)

	for iter != nil && filter(iter.Tx()) {
		iter = iter.Next()
	}
}

// buildIterator ensures that EVM mempool has checked txs for reorgs up to COMMITTED
// block height and then returns a combined iterator over EVM & Cosmos txs.
func (m *Mempool) buildIterator(ctx context.Context, txs [][]byte) sdkmempool.Iterator {
	defer func(t0 time.Time) { telemetry.MeasureSince(t0, "expmempool_builditerator_duration") }(time.Now()) //nolint:staticcheck

	evmIterator, cosmosIterator := m.getIterators(ctx, txs)

	return NewEVMMempoolIterator(
		evmIterator,
		cosmosIterator,
		m.logger,
		m.txConfig,
		m.blockchain,
	)
}

// CountTx returns the total number of transactions in both EVM and Cosmos pools.
// This provides a combined count across all mempool types.
func (m *Mempool) CountTx() int {
	pending, _ := m.txPool.Stats()
	return m.recheckCosmosPool.CountTx() + pending
}

// Remove fallbacks for RemoveWithReason
func (m *Mempool) Remove(tx sdk.Tx) error {
	return m.RemoveWithReason(context.Background(), tx, sdkmempool.RemoveReason{
		Caller: "remove",
		Error:  nil,
	})
}

// RemoveWithReason removes a transaction from the appropriate sdkmempool.
//
// NOTE: even if removal fails, side effects may have occurred like recording
// nonce increments.
func (m *Mempool) RemoveWithReason(_ context.Context, tx sdk.Tx, reason sdkmempool.RemoveReason) error {
	msgEthereumTx, err := evmTxFromCosmosTx(tx)
	switch {
	case errors.Is(err, ErrNoMessages):
		return err
	case err != nil:
		// unable to parse evm tx -> process as cosmos tx
		return m.removeCosmosTx(tx, reason)
	default:
		return m.removeEVMTx(tx, msgEthereumTx, reason)
	}
}

// removeCosmosTx removes a cosmos tx from the mempool.
// The RecheckMempool handles locking internally.
func (m *Mempool) removeCosmosTx(tx sdk.Tx, reason sdkmempool.RemoveReason) error {
	m.logger.Debug("Removing Cosmos transaction")

	if reason.Caller == sdkmempool.CallerRunTxFinalize {
		m.recordNonceAdvances(tx)
	}

	if err := m.recheckCosmosPool.Remove(tx); err != nil {
		m.logger.Error("Failed to remove Cosmos transaction", "error", err)
		return fmt.Errorf("removing cosmos tx: %w", err)
	}

	m.logger.Debug("Cosmos transaction removed successfully")

	return nil
}

// removeEVMTx removes a evm tx from the mempool.
func (m *Mempool) removeEVMTx(tx sdk.Tx, msgEthereumTx *evmtypes.MsgEthereumTx, reason sdkmempool.RemoveReason) error {
	m.logger.Debug("Removing EVM transaction")

	hash := msgEthereumTx.AsTransaction().Hash()
	if reason.Caller == sdkmempool.CallerRunTxFinalize {
		_ = m.txTracker.IncludedInBlock(hash)
		m.recordNonceAdvances(tx)
	}

	if m.shouldRemoveFromEVMPool(hash, reason) {
		m.logger.Debug("Manually removing EVM transaction", "tx_hash", hash)
		m.legacyTxPool.RemoveTx(hash, false, true, convertRemovalReason(reason.Caller))
	}

	m.logger.Debug("EVM transaction removed successfully")

	return nil
}

// recordNonceAdvances records the on chain nonce advance for every signer of
// tx in the legacypool.
func (m *Mempool) recordNonceAdvances(tx sdk.Tx) {
	signers, err := m.signerExtractor.GetSigners(tx)
	if err != nil {
		m.logger.Warn("could not extract signers for nonce tracking", "err", err)
		return
	}
	for _, s := range signers {
		m.legacyTxPool.SetLatestNonce(common.BytesToAddress(s.Signer), s.Sequence)
	}
}

// shouldRemoveFromEVMPool determines whether an EVM transaction should be manually removed.
func (m *Mempool) shouldRemoveFromEVMPool(hash common.Hash, reason sdkmempool.RemoveReason) bool {
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
func (m *Mempool) SetEventBus(eventBus *cmttypes.EventBus) {
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
func (m *Mempool) HasEventBus() bool {
	return m.eventBus != nil
}

func (m *Mempool) Close() error {
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
func (m *Mempool) getIterators(ctx context.Context, _ [][]byte) (evm *miner.TransactionsByPriceAndNonce, cosmos sdkmempool.Iterator) {
	var (
		evmIterator    *miner.TransactionsByPriceAndNonce
		cosmosIterator sdkmempool.Iterator
		wg             sync.WaitGroup
	)

	// using ctx.BlockHeight() - 1 since we want to get txs that have been
	// validated at latest committed height, and ctx.BlockHeight() returns the
	// latest uncommitted height
	sdkctx := sdk.UnwrapSDKContext(ctx)
	selectHeight := new(big.Int).SetInt64(sdkctx.BlockHeight() - 1)

	// Keeper reads consume gas on the SDK context. Fetch these inputs once
	// before starting goroutines so we do not race on the shared gas meters.
	baseFee := m.vmKeeper.GetBaseFee(sdkctx)
	coinDenom := m.blockchain.GetCoinDenom()
	cosmosBaseFee := currentBaseFee(m.blockchain)

	wg.Go(func() {
		evmIterator = m.evmIterator(ctx, selectHeight, baseFee)
	})

	wg.Go(func() {
		cosmosIterator = m.cosmosIterator(ctx, selectHeight, coinDenom, cosmosBaseFee)
	})

	wg.Wait()

	return evmIterator, cosmosIterator
}

// evmIterator returns an iterator over the current valid txs in the evm
// mempool at height.
func (m *Mempool) evmIterator(ctx context.Context, height *big.Int, baseFee *big.Int) *miner.TransactionsByPriceAndNonce {
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
func (m *Mempool) cosmosIterator(
	ctx context.Context,
	height *big.Int,
	bondDenom string,
	baseFee *uint256.Int,
) sdkmempool.Iterator {
	if m.pendingTxProposalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.pendingTxProposalTimeout)
		defer cancel()
	}
	return m.recheckCosmosPool.OrderedRecheckedTxs(ctx, height, bondDenom, baseFee)
}

// TrackTx submits a tx to be tracked for its tx inclusion metrics.
func (m *Mempool) TrackTx(hash common.Hash) error {
	return m.txTracker.Track(hash)
}

// RecheckEVMTxs triggers a synchronous recheck of evm transactions.
// This should only be used for testing.
func (m *Mempool) RecheckEVMTxs(newHead *ethtypes.Header) {
	m.txPool.Reset(nil, newHead)
}

// RecheckCosmosTxs triggers a synchronous recheck of cosmos transactions.
// This should only used for testing.
func (m *Mempool) RecheckCosmosTxs(newHead *ethtypes.Header) {
	m.recheckCosmosPool.TriggerRecheckSync(newHead)
}

// getEVMMessage validates that the transaction contains exactly one message and returns it if it's an EVM message.
// Returns an error if the transaction has no messages, multiple messages, or the single message is not an EVM transaction.
func evmTxFromCosmosTx(tx sdk.Tx) (*evmtypes.MsgEthereumTx, error) {
	msgs := tx.GetMsgs()
	if len(msgs) == 0 {
		return nil, ErrNoMessages
	}

	// ethereum txs should only contain a single msg that is a MsgEthereumTx
	// type
	if len(msgs) > 1 {
		// transaction has > 1 msg, will be treated as a cosmos tx by the
		// mempool. validate that none of the msgs are a MsgEthereumTx since
		// those should only be used in the single msg case
		for _, msg := range msgs {
			if _, ok := msg.(*evmtypes.MsgEthereumTx); ok {
				return nil, ErrMultiMsgEthereumTransaction
			}
		}

		// transaction has > 1 msg, but none were ethereum txs, this is
		// still not a valid eth tx
		return nil, fmt.Errorf("%w, got %d", ErrExpectedOneMessage, len(msgs))
	}

	ethMsg, ok := msgs[0].(*evmtypes.MsgEthereumTx)
	if !ok {
		return nil, ErrNotEVMTransaction
	}
	return ethMsg, nil
}

// convertRemovalReason converts a removal caller to a removal reason
func convertRemovalReason(caller sdkmempool.RemovalCaller) txpool.RemovalReason {
	switch caller {
	case sdkmempool.CallerRunTxRecheck:
		return legacypool.RemovalReasonRunTxRecheck
	case sdkmempool.CallerRunTxFinalize:
		return legacypool.RemovalReasonRunTxFinalize
	case sdkmempool.CallerPrepareProposalRemoveInvalid:
		return legacypool.RemovalReasonPrepareProposalInvalid
	default:
		return txpool.RemovalReason("")
	}
}
