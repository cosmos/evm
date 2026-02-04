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

	"github.com/cosmos/evm/mempool/miner"
	"github.com/cosmos/evm/mempool/reserver"
	"github.com/cosmos/evm/mempool/txpool"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/rpc/stream"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

var _ sdkmempool.ExtMempool = &ExperimentalEVMMempool{}

const (
	// SubscriberName is the name of the event bus subscriber for the EVM mempool
	SubscriberName = "evm"
	// fallbackBlockGasLimit is the default block gas limit is 0 or missing in genesis file
	fallbackBlockGasLimit = 100_000_000
)

type (
	// ExperimentalEVMMempool is a unified mempool that manages both EVM and Cosmos SDK transactions.
	// It provides a single interface for transaction insertion, selection, and removal while
	// maintaining separate pools for EVM and Cosmos transactions. The mempool handles
	// fee-based transaction prioritization and manages nonce sequencing for EVM transactions.
	ExperimentalEVMMempool struct {
		/** Keepers **/
		vmKeeper VMKeeperI

		cosmosReserver *reserver.ReservationHandle

		/** Mempools **/
		txPool                   *txpool.TxPool
		legacyTxPool             *legacypool.LegacyPool
		cosmosPool               sdkmempool.ExtMempool
		operateExclusively       bool
		pendingTxProposalTimeout time.Duration

		/** Utils **/
		logger        log.Logger
		txConfig      client.TxConfig
		blockchain    *Blockchain
		blockGasLimit uint64 // Block gas limit from consensus parameters
		minTip        *uint256.Int

		/** Verification **/
		anteHandler sdk.AnteHandler

		/** Concurrency **/
		mu sync.Mutex

		eventBus *cmttypes.EventBus

		/** Transaction Reaping **/
		reapList *ReapList

		/** Transaction Tracking **/
		txTracker *txTracker

		/** Transaction Inserting **/
		iq *insertQueue
	}
)

// EVMMempoolConfig contains configuration options for creating an EVMsdkmempool.
// It allows customization of the underlying mempools, verification functions,
// and broadcasting functions used by the sdkmempool.
type EVMMempoolConfig struct {
	LegacyPoolConfig *legacypool.Config
	CosmosPoolConfig *sdkmempool.PriorityNonceMempoolConfig[math.Int]
	AnteHandler      sdk.AnteHandler
	// Block gas limit from consensus parameters
	BlockGasLimit uint64
	MinTip        *uint256.Int
	// OperateExclusively indicates whether this mempool is the ONLY mempool in the chain.
	// If false, comet-bft also operates its own clist-mempool. If true, then the mempool expects exclusive
	// handling of transactions via ABCI.InsertTx & ABCI.ReapTxs.
	OperateExclusively bool
	// PendingTxProposalTimeout is the max amount of time to allocate to
	// fetching (or watiing to fetch) pending txs from the evm mempool.
	PendingTxProposalTimeout time.Duration
}

// NewExperimentalEVMMempool creates a new unified mempool for EVM and Cosmos transactions.
// It initializes both EVM and Cosmos transaction pools, sets up blockchain interfaces,
// and configures fee-based prioritization. The config parameter allows customization
// of pools and verification functions, with sensible defaults created if not provided.
func NewExperimentalEVMMempool(
	getCtxCallback func(height int64, prove bool) (sdk.Context, error),
	logger log.Logger,
	vmKeeper VMKeeperI,
	feeMarketKeeper FeeMarketKeeperI,
	txConfig client.TxConfig,
	clientCtx client.Context,
	txEncoder *TxEncoder,
	rechecker legacypool.Rechecker,
	config *EVMMempoolConfig,
	cosmosPoolMaxTx int,
) *ExperimentalEVMMempool {
	var (
		cosmosPool sdkmempool.ExtMempool
		blockchain *Blockchain
	)

	// add the mempool name to the logger
	logger = logger.With(log.ModuleKey, "ExperimentalEVMMempool")

	logger.Debug("creating new EVM mempool")

	if config == nil {
		panic("config must not be nil")
	}

	if config.BlockGasLimit == 0 {
		logger.Warn("block gas limit is 0, setting to fallback", "fallback_limit", fallbackBlockGasLimit)
		config.BlockGasLimit = fallbackBlockGasLimit
	}

	blockchain = NewBlockchain(getCtxCallback, logger, vmKeeper, feeMarketKeeper, config.BlockGasLimit)

	// Create txPool from configuration
	legacyConfig := legacypool.DefaultConfig
	if config.LegacyPoolConfig != nil {
		legacyConfig = *config.LegacyPoolConfig
	}
	legacyPool := legacypool.New(
		legacyConfig,
		blockchain,
		legacypool.WithRecheck(rechecker),
	)

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

	// TODO: move this logic to evmd.createMempoolConfig and set the max tx there
	// Create Cosmos Mempool from configuration
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
	cosmosPool = sdkmempool.NewPriorityMempool(*cosmosPoolConfig)

	evmMempool := &ExperimentalEVMMempool{
		vmKeeper:                 vmKeeper,
		cosmosReserver:           tracker.NewHandle(-1),
		txPool:                   txPool,
		legacyTxPool:             txPool.Subpools[0].(*legacypool.LegacyPool),
		cosmosPool:               cosmosPool,
		logger:                   logger,
		txConfig:                 txConfig,
		blockchain:               blockchain,
		blockGasLimit:            config.BlockGasLimit,
		minTip:                   config.MinTip,
		anteHandler:              config.AnteHandler,
		operateExclusively:       config.OperateExclusively,
		pendingTxProposalTimeout: config.PendingTxProposalTimeout,
		reapList:                 NewReapList(txEncoder),
		txTracker:                newTxTracker(),
		iq:                       newInsertQueue(legacyPool, logger),
	}

	// Once we have validated that the tx is valid (and can be promoted, set it
	// to be reaped)
	legacyPool.OnTxPromoted = func(tx *ethtypes.Transaction) {
		if err := evmMempool.reapList.PushEVMTx(tx); err != nil {
			logger.Error("could not push evm tx to ReapList", "err", err)
		}

		hash := tx.Hash()
		_ = evmMempool.txTracker.ExitedQueued(hash)
		_ = evmMempool.txTracker.EnteredPending(hash)
	}

	legacyPool.OnTxEnqueued = func(tx *ethtypes.Transaction) {
		_ = evmMempool.txTracker.EnteredQueued(tx.Hash())
	}

	// Once we are removing the tx, we no longer need to block it from being
	// sent to the reaplist again and can remove from the guard
	legacyPool.OnTxRemoved = func(tx *ethtypes.Transaction, pool legacypool.PoolType) {
		// tx was invalidated for some reason or was included in a block
		// (either way it is no longer in the mempool), if this tx is in the
		// reap list we need remove it from there (no longer need to gossip to
		// others about the tx) + the reap guard (since we may see this tx at a
		// later time, in which case we should gossip it again) by readding to
		// the reap guard.
		evmMempool.reapList.DropEVMTx(tx)

		_ = evmMempool.txTracker.RemoveTx(tx.Hash(), pool)
	}

	vmKeeper.SetEvmMempool(evmMempool)

	return evmMempool
}

// IsExclusive returns true if this mempool is the ONLY mempool in the chain.
func (m *ExperimentalEVMMempool) IsExclusive() bool {
	return m.operateExclusively
}

// GetBlockchain returns the blockchain interface used for chain head event notifications.
// This is primarily used to notify the mempool when new blocks are finalized.
func (m *ExperimentalEVMMempool) GetBlockchain() *Blockchain {
	return m.blockchain
}

// GetTxPool returns the underlying EVM txpool.
// This provides direct access to the EVM-specific transaction management functionality.
func (m *ExperimentalEVMMempool) GetTxPool() *txpool.TxPool {
	return m.txPool
}

// Insert adds a transaction to the appropriate mempool (EVM or Cosmos).
// EVM transactions are routed to the EVM transaction pool, while all other
// transactions are inserted into the Cosmos sdkmempool.
func (m *ExperimentalEVMMempool) Insert(ctx context.Context, tx sdk.Tx) error {
	return m.insert(ctx, tx, true)
}

// InsertAsync adds a transaction to the appropriate mempool (EVM or Cosmos). EVM
// transactions are routed to the EVM transaction pool, while all other
// transactions are inserted into the Cosmos sdkmempool. EVM transactions are
// inserted async, i.e. they are scheduled for promotion only, we do not wait
// for it to complete.
func (m *ExperimentalEVMMempool) InsertAsync(ctx context.Context, tx sdk.Tx) error {
	return m.insert(ctx, tx, false)
}

func (m *ExperimentalEVMMempool) insert(ctx context.Context, tx sdk.Tx, sync bool) error {
	ethMsg, err := evmTxFromCosmosTx(tx)
	if err == nil {
		ethTx := ethMsg.AsTransaction()
		if !sync {
			m.iq.Push(ethTx)
			return nil
		}

		errs := m.txPool.Add([]*ethtypes.Transaction{ethTx}, sync)
		if len(errs) != 1 {
			panic(fmt.Errorf("expected a single error when compacting evm tx add errors"))
		}
		if errs[0] != nil {
			m.logger.Error("error inserting evm tx into pool", "tx_hash", ethTx.Hash(), "err", errs[0])
		}
		return errs[0]
	case errors.Is(err, ErrMultiMsgEthereumTransaction):
		// there are multiple messages in this tx and one or more of them is an
		// evm tx, this is invalid
		return err
	default:
		// tx either has no messages, or has a single non MsgEtherumTx msg, or
		// has multiple msgs, where none are MsgEthereumTx
		return m.insertCosmosTx(ctx, tx)
	}
}

// insertCosmosTx inserts a cosmos tx into the cosmos mempool. This also
// performs a CheckTx (anteHandler) call in the hot path.
func (m *ExperimentalEVMMempool) insertCosmosTx(goCtx context.Context, tx sdk.Tx) error {
	// we have to process cosmos txs serially.
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx := sdk.UnwrapSDKContext(goCtx)

	// Insert into cosmos pool for non-EVM transactions
	m.logger.Debug("inserting Cosmos transaction")

	// NOTE: this is a check tx back in the hot path of comet and will slow
	// down the insert, however for our initial purposes we do not plan to have
	// many (if any) cosmos txs, so we are accepting this limitation for now
	// for simplicity.

	// copying context/ms branching done in runTx

	// get the current multistore in the context
	ms := ctx.MultiStore()

	// branch the multistore into so we have a place to make anteHandler writes
	// without messing up the original state in case the anteHandler sequence
	// fails
	msCache := ms.CacheMultiStore()

	// set the branched multistore as the multistore that the context will use.
	// so writes happening via this context will use the branched multistore.
	ctx = ctx.WithMultiStore(msCache)

	// execute the anteHandlers on our new context, and get a context that has
	// the anteHandler updates written to it.
	if _, err := m.anteHandler(ctx, tx, false); err != nil {
		return fmt.Errorf("running anteHandler sequence for tx: %w", err)
	}

	// anteHandler has successfully completed, write its updates that are
	// sitting in the branched multistore, back to their parent multistore.
	// After this we will have updated the parent state and the next
	// anteHandler invocation using this state will build off its updates.
	msCache.Write()

	// Extract signer addresses and convert to EVM addresses
	evmAddrs, err := signerAddressesFromSDKTx(tx)
	if err != nil {
		return err
	}
	if err := m.cosmosReserver.Hold(evmAddrs...); err != nil {
		return err
	}

	if err := m.cosmosPool.Insert(goCtx, tx); err != nil {
		m.logger.Error("failed to insert Cosmos transaction", "error", err)
		m.cosmosReserver.Release(evmAddrs...) //nolint:errcheck // ignoring is fine here.
		return err
	}

	m.logger.Debug("Cosmos transaction inserted successfully")
	if err := m.reapList.PushCosmosTx(tx); err != nil {
		panic(fmt.Errorf("successfully inserted cosmos tx, but failed to insert into reap list: %w", err))
	}
	return nil
}

func signerAddressesFromSDKTx(tx sdk.Tx) ([]common.Address, error) {
	var signerAddrs []common.Address
	if sigTx, ok := tx.(interface{ GetSigners() ([][]byte, error) }); ok {
		signers, err := sigTx.GetSigners()
		if err != nil {
			return nil, err
		}
		for _, addr := range signers {
			signerAddrs = append(signerAddrs, common.BytesToAddress(addr))
		}
	}
	if len(signerAddrs) == 0 {
		return nil, fmt.Errorf("tx contains no signers")
	}
	return signerAddrs, nil
}

// InsertInvalidNonce handles transactions that failed with nonce gap errors.
// It attempts to insert EVM transactions into the pool as non-local transactions,
// allowing them to be queued for future execution when the nonce gap is filled.
// Non-EVM transactions are discarded as regular Cosmos flows do not support nonce gaps.
func (m *ExperimentalEVMMempool) InsertInvalidNonce(txBytes []byte) error {
	tx, err := m.txConfig.TxDecoder()(txBytes)
	if err != nil {
		return err
	}

	var ethTxs []*ethtypes.Transaction
	msgs := tx.GetMsgs()
	if len(msgs) != 1 {
		return fmt.Errorf("%w, got %d", ErrExpectedOneMessage, len(msgs))
	}
	for _, msg := range tx.GetMsgs() {
		ethMsg, ok := msg.(*evmtypes.MsgEthereumTx)
		if ok {
			ethTxs = append(ethTxs, ethMsg.AsTransaction())
			continue
		}
	}
	errs := m.txPool.Add(ethTxs, false)
	if errs != nil {
		if len(errs) != 1 {
			return fmt.Errorf("%w, got %d", ErrExpectedOneError, len(errs))
		}
		return errs[0]
	}
	return nil
}

// ReapNewValidTxs removes and returns the oldest transactions from the reap
// list until maxBytes or maxGas limits are reached.
func (m *ExperimentalEVMMempool) ReapNewValidTxs(maxBytes uint64, maxGas uint64) ([][]byte, error) {
	m.logger.Debug("reaping transactions", "maxBytes", maxBytes, "maxGas", maxGas, "available_txs")
	txs := m.reapList.Reap(maxBytes, maxGas)
	m.logger.Debug("reap complete", "txs_reaped", len(txs))

	return txs, nil
}

// Select returns a unified iterator over both EVM and Cosmos transactions.
// The iterator prioritizes transactions based on their fees and manages proper
// sequencing. The i parameter contains transaction hashes to exclude from selection.
func (m *ExperimentalEVMMempool) Select(goCtx context.Context, i [][]byte) sdkmempool.Iterator {
	return m.buildIterator(goCtx, i)
}

// SelectBy iterates through transactions until the provided filter function returns false.
// It uses the same unified iterator as Select but allows early termination based on
// custom criteria defined by the filter function.
func (m *ExperimentalEVMMempool) SelectBy(goCtx context.Context, txs [][]byte, filter func(sdk.Tx) bool) {
	defer func(t0 time.Time) { telemetry.MeasureSince(t0, "expmempool_selectby_duration") }(time.Now()) //nolint:staticcheck

	iter := m.buildIterator(goCtx, txs)

	for iter != nil && filter(iter.Tx()) {
		iter = iter.Next()
	}
}

// buildIterator ensures that EVM mempool has checked txs for reorgs up to COMMITTED
// block height and then returns a combined iterator over EVM & Cosmos txs.
func (m *ExperimentalEVMMempool) buildIterator(ctx context.Context, txs [][]byte) sdkmempool.Iterator {
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
func (m *ExperimentalEVMMempool) CountTx() int {
	pending, _ := m.txPool.Stats()
	return m.cosmosPool.CountTx() + pending
}

// Remove fallbacks for RemoveWithReason
func (m *ExperimentalEVMMempool) Remove(tx sdk.Tx) error {
	return m.RemoveWithReason(context.Background(), tx, sdkmempool.RemoveReason{
		Caller: "remove",
		Error:  nil,
	})
}

// RemoveWithReason removes a transaction from the appropriate sdkmempool.
// For EVM transactions, removal is typically handled automatically by the pool
// based on nonce progression. Cosmos transactions are removed from the Cosmos pool.
func (m *ExperimentalEVMMempool) RemoveWithReason(ctx context.Context, tx sdk.Tx, reason sdkmempool.RemoveReason) error {
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

// convertRemovalReason converts a removal caller to a removal reason
func convertRemovalReason(caller sdkmempool.RemovalCaller) txpool.RemovalReason {
	switch caller {
	case sdkmempool.CallerRunTxRecheck:
		return legacypool.RemovalReasonRunTxRecheck
	case sdkmempool.CallerRunTxFinalize:
		return legacypool.RemovalReasonRunTxFinalize
	case sdkmempool.CallerPrepareProposalRemoveInvalid:
		return legacypool.RemovalReasonPreparePropsoalInvalid
	default:
		return txpool.RemovalReason("")
	}
}

// caller should hold the lock
func (m *ExperimentalEVMMempool) removeCosmosTx(ctx context.Context, tx sdk.Tx, reason sdkmempool.RemoveReason) error {
	m.logger.Debug("Removing Cosmos transaction")

	err := sdkmempool.RemoveWithReason(ctx, m.cosmosPool, tx, reason)
	if err != nil {
		m.logger.Error("Failed to remove Cosmos transaction", "error", err)
		return err
	}

	m.reapList.DropCosmosTx(tx)
	m.logger.Debug("Cosmos transaction removed successfully")

	evmAddrs, err := signerAddressesFromSDKTx(tx)
	if err != nil {
		return err
	}
	m.cosmosReserver.Release(evmAddrs...) //nolint:errcheck // ignoring is fine here.

	return nil
}

// shouldRemoveFromEVMPool determines whether an EVM transaction should be manually removed.
func (m *ExperimentalEVMMempool) shouldRemoveFromEVMPool(hash common.Hash, reason sdkmempool.RemoveReason) bool {
	if reason.Error == nil {
		return true
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
func (m *ExperimentalEVMMempool) SetEventBus(eventBus *cmttypes.EventBus) {
	if m.HasEventBus() {
		m.eventBus.Unsubscribe(context.Background(), SubscriberName, stream.NewBlockHeaderEvents) //nolint: errcheck
	}
	m.eventBus = eventBus
	sub, err := eventBus.Subscribe(context.Background(), SubscriberName, stream.NewBlockHeaderEvents)
	if err != nil {
		panic(err)
	}
	go func() {
		for range sub.Out() {
			m.GetBlockchain().NotifyNewBlock()
		}
	}()
}

// HasEventBus returns true if the blockchain is configured to use an event bus for block notifications.
func (m *ExperimentalEVMMempool) HasEventBus() bool {
	return m.eventBus != nil
}

// Close unsubscribes from the CometBFT event bus and shuts down the mempool.
func (m *ExperimentalEVMMempool) Close() error {
	var errs []error
	if m.eventBus != nil {
		if err := m.eventBus.Unsubscribe(context.Background(), SubscriberName, stream.NewBlockHeaderEvents); err != nil {
			errs = append(errs, fmt.Errorf("failed to unsubscribe from event bus: %w", err))
		}
	}

	m.iq.Close()

	if err := m.txPool.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close txpool: %w", err))
	}

	return errors.Join(errs...)
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

// getIterators prepares iterators over pending EVM and Cosmos transactions.
// It configures EVM transactions with proper base fee filtering and priority ordering,
// while setting up the Cosmos iterator with the provided exclusion list.
func (m *ExperimentalEVMMempool) getIterators(ctx context.Context, txs [][]byte) (*miner.TransactionsByPriceAndNonce, sdkmempool.Iterator) {
	sdkctx := sdk.UnwrapSDKContext(ctx)
	baseFee := m.vmKeeper.GetBaseFee(sdkctx)
	var baseFeeUint *uint256.Int
	if baseFee != nil {
		baseFeeUint = uint256.MustFromBig(baseFee)
	}

	if m.pendingTxProposalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.pendingTxProposalTimeout)
		defer cancel()
	}

	filter := txpool.PendingFilter{
		MinTip:       m.minTip,
		BaseFee:      baseFeeUint,
		BlobFee:      nil,
		OnlyPlainTxs: true,
		OnlyBlobTxs:  false,
	}
	evmPendingTxs := m.txPool.Pending(ctx, new(big.Int).SetInt64(sdkctx.BlockHeight()-1), filter)
	evmIterator := miner.NewTransactionsByPriceAndNonce(nil, evmPendingTxs, baseFee)
	cosmosIterator := m.cosmosPool.Select(ctx, txs)

	return evmIterator, cosmosIterator
}

func (m *ExperimentalEVMMempool) TrackTx(hash common.Hash) error {
	return m.txTracker.Track(hash)
}
