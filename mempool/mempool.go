package mempool

import (
	"context"
	"errors"
	"fmt"
	"sync"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/evm/mempool/miner"
	"github.com/cosmos/evm/mempool/txpool"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/rpc/stream"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
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

		/** Mempools **/
		txPool       *txpool.TxPool
		legacyTxPool *legacypool.LegacyPool
		cosmosPool   sdkmempool.ExtMempool

		/** Utils **/
		logger        log.Logger
		txConfig      client.TxConfig
		blockchain    *Blockchain
		blockGasLimit uint64 // Block gas limit from consensus parameters
		minTip        *uint256.Int

		/** Verification **/
		anteHandler sdk.AnteHandler

		/** Concurrency **/
		mtx sync.Mutex

		eventBus *cmttypes.EventBus
	}
)

// EVMMempoolConfig contains configuration options for creating an EVMsdkmempool.
// It allows customization of the underlying mempools, verification functions,
// and broadcasting functions used by the sdkmempool.
type EVMMempoolConfig struct {
	LegacyPoolConfig *legacypool.Config
	CosmosPoolConfig *sdkmempool.PriorityNonceMempoolConfig[math.Int]
	AnteHandler      sdk.AnteHandler
	BroadCastTxFn    func(txs []*ethtypes.Transaction) error
	BlockGasLimit    uint64 // Block gas limit from consensus parameters
	MinTip           *uint256.Int
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

	legacyPool := legacypool.New(legacyConfig, blockchain)

	// Set up broadcast function using clientCtx
	if config.BroadCastTxFn != nil {
		legacyPool.BroadcastTxFn = config.BroadCastTxFn
	} else {
		// Create default broadcast function using clientCtx.
		// The EVM mempool will broadcast transactions when it promotes them
		// from queued into pending, noting their readiness to be executed.
		legacyPool.BroadcastTxFn = func(txs []*ethtypes.Transaction) error {
			logger.Debug("broadcasting EVM transactions", "tx_count", len(txs))
			return broadcastEVMTransactions(clientCtx, txConfig, txs)
		}
	}

	txPool, err := txpool.New(uint64(0), blockchain, []txpool.SubPool{legacyPool})
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
		vmKeeper:      vmKeeper,
		txPool:        txPool,
		legacyTxPool:  txPool.Subpools[0].(*legacypool.LegacyPool),
		cosmosPool:    cosmosPool,
		logger:        logger,
		txConfig:      txConfig,
		blockchain:    blockchain,
		blockGasLimit: config.BlockGasLimit,
		minTip:        config.MinTip,
		anteHandler:   config.AnteHandler,
	}

	vmKeeper.SetEvmMempool(evmMempool)

	return evmMempool
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
// transactions are inserted into the Cosmos sdkmempool. The method assumes
// transactions have already passed CheckTx validation.
func (m *ExperimentalEVMMempool) Insert(goCtx context.Context, tx sdk.Tx) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	ctx := sdk.UnwrapSDKContext(goCtx)
	blockHeight := ctx.BlockHeight()

	m.logger.Debug("inserting transaction into mempool", "block_height", blockHeight)
	ethMsg, err := m.getEVMMessage(tx)
	if err == nil {
		// Insert into EVM pool
		hash := ethMsg.Hash()
		m.logger.Debug("inserting EVM transaction", "tx_hash", hash)
		ethTxs := []*ethtypes.Transaction{ethMsg.AsTransaction()}
		errs := m.txPool.Add(ethTxs, true)
		if len(errs) > 0 && errs[0] != nil {
			m.logger.Error("failed to insert EVM transaction", "error", errs[0], "tx_hash", hash)
			return errs[0]
		}
		m.logger.Debug("EVM transaction inserted successfully", "tx_hash", hash)
		return nil
	}

	// Insert into cosmos pool for non-EVM transactions
	m.logger.Debug("inserting Cosmos transaction", "error", err)
	err = m.cosmosPool.Insert(goCtx, tx)
	if err != nil {
		m.logger.Error("failed to insert Cosmos transaction", "error", err)
	} else {
		m.logger.Debug("Cosmos transaction inserted successfully")
	}
	return err
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

// Select returns a unified iterator over both EVM and Cosmos transactions.
// The iterator prioritizes transactions based on their fees and manages proper
// sequencing. The i parameter contains transaction hashes to exclude from selection.
func (m *ExperimentalEVMMempool) Select(goCtx context.Context, i [][]byte) sdkmempool.Iterator {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	ctx := sdk.UnwrapSDKContext(goCtx)

	evmIterator, cosmosIterator := m.getIterators(goCtx, i)

	combinedIterator := NewEVMMempoolIterator(evmIterator, cosmosIterator, m.logger, m.txConfig, m.vmKeeper.GetEvmCoinInfo(ctx).Denom, m.blockchain.Config().ChainID, m.blockchain)

	return combinedIterator
}

// CountTx returns the total number of transactions in both EVM and Cosmos pools.
// This provides a combined count across all mempool types.
func (m *ExperimentalEVMMempool) CountTx() int {
	pending, _ := m.txPool.Stats()
	return m.cosmosPool.CountTx() + pending
}

// Remove removes a transaction from the appropriate sdkmempool.
// For EVM transactions, removal is typically handled automatically by the pool
// based on nonce progression. Cosmos transactions are removed from the Cosmos pool.
func (m *ExperimentalEVMMempool) Remove(tx sdk.Tx) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	if m.blockchain.latestCtx.BlockHeight() == 0 {
		return nil
	}

	m.logger.Debug("removing transaction from mempool")

	msg, err := m.getEVMMessage(tx)
	if err == nil {
		// Comet will attempt to remove transactions from the mempool after completing successfully.
		// We should not do this with EVM transactions because removing them causes the subsequent ones to
		// be dequeued as temporarily invalid, only to be requeued a block later.
		// The EVM mempool handles removal based on account nonce automatically.
		hash := msg.Hash()
		if m.shouldRemoveFromEVMPool(tx) {
			m.logger.Debug("manually removing EVM transaction", "tx_hash", hash)
			m.legacyTxPool.RemoveTx(hash, false, true)
		} else {
			m.logger.Debug("skipping manual removal of EVM transaction, leaving to mempool to handle", "tx_hash", hash)
		}
		return nil
	}

	if errors.Is(err, ErrNoMessages) {
		return err
	}

	m.logger.Debug("removing Cosmos transaction")
	err = m.cosmosPool.Remove(tx)
	if err != nil {
		m.logger.Error("failed to remove Cosmos transaction", "error", err)
	} else {
		m.logger.Debug("Cosmos transaction removed successfully")
	}
	return err
}

// shouldRemoveFromEVMPool determines whether an EVM transaction should be manually removed.
// It uses the AnteHandler to check if the transaction failed for reasons
// other than nonce gaps or successful execution, in which case manual removal is needed.
func (m *ExperimentalEVMMempool) shouldRemoveFromEVMPool(tx sdk.Tx) bool {
	if m.anteHandler == nil {
		m.logger.Debug("no ante handler available, keeping transaction")
		return false
	}

	// If it was a successful transaction or a sequence error, we let the mempool handle the cleaning.
	// If it was any other Cosmos or antehandler related issue, then we remove it.
	ctx, err := m.blockchain.GetLatestContext()
	if err != nil {
		m.logger.Debug("cannot get latest context for validation, keeping transaction", "error", err)
		return false // Cannot validate, keep transaction
	}

	_, err = m.anteHandler(ctx, tx, true)
	// Keep nonce gap transactions, remove others that fail validation
	if errors.Is(err, ErrNonceGap) || errors.Is(err, sdkerrors.ErrInvalidSequence) || errors.Is(err, sdkerrors.ErrOutOfGas) {
		m.logger.Debug("nonce gap detected, keeping transaction", "error", err)
		return false
	}

	if err != nil {
		m.logger.Debug("transaction validation failed, should be removed", "error", err)
	} else {
		m.logger.Debug("transaction validation succeeded, should be kept")
	}

	return err != nil
}

// SelectBy iterates through transactions until the provided filter function returns false.
// It uses the same unified iterator as Select but allows early termination based on
// custom criteria defined by the filter function.
func (m *ExperimentalEVMMempool) SelectBy(goCtx context.Context, i [][]byte, f func(sdk.Tx) bool) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	ctx := sdk.UnwrapSDKContext(goCtx)

	evmIterator, cosmosIterator := m.getIterators(goCtx, i)

	combinedIterator := NewEVMMempoolIterator(evmIterator, cosmosIterator, m.logger, m.txConfig, m.vmKeeper.GetEvmCoinInfo(ctx).Denom, m.blockchain.Config().ChainID, m.blockchain)

	for combinedIterator != nil && f(combinedIterator.Tx()) {
		combinedIterator = combinedIterator.Next()
	}
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

	if err := m.txPool.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close txpool: %w", err))
	}

	return errors.Join(errs...)
}

// getEVMMessage validates that the transaction contains exactly one message and returns it if it's an EVM message.
// Returns an error if the transaction has no messages, multiple messages, or the single message is not an EVM transaction.
func (m *ExperimentalEVMMempool) getEVMMessage(tx sdk.Tx) (*evmtypes.MsgEthereumTx, error) {
	msgs := tx.GetMsgs()
	if len(msgs) == 0 {
		return nil, ErrNoMessages
	}
	if len(msgs) != 1 {
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
func (m *ExperimentalEVMMempool) getIterators(goCtx context.Context, i [][]byte) (*miner.TransactionsByPriceAndNonce, sdkmempool.Iterator) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	baseFee := m.vmKeeper.GetBaseFee(ctx)
	var baseFeeUint *uint256.Int
	if baseFee != nil {
		baseFeeUint = uint256.MustFromBig(baseFee)
	}

	m.logger.Debug("getting iterators")

	pendingFilter := txpool.PendingFilter{
		MinTip:       m.minTip,
		BaseFee:      baseFeeUint,
		BlobFee:      nil,
		OnlyPlainTxs: true,
		OnlyBlobTxs:  false,
	}
	evmPendingTxes := m.txPool.Pending(pendingFilter)
	orderedEVMPendingTxes := miner.NewTransactionsByPriceAndNonce(nil, evmPendingTxes, baseFee)

	cosmosPendingTxes := m.cosmosPool.Select(ctx, i)

	return orderedEVMPendingTxes, cosmosPendingTxes
}

// broadcastEVMTransactions converts Ethereum transactions to Cosmos SDK format and broadcasts them.
// This function wraps EVM transactions in MsgEthereumTx messages and submits them to the network
// using the provided client context. It handles encoding and error reporting for each transaction.
func broadcastEVMTransactions(clientCtx client.Context, txConfig client.TxConfig, ethTxs []*ethtypes.Transaction) error {
	for _, ethTx := range ethTxs {
		msg := &evmtypes.MsgEthereumTx{}
		msg.FromEthereumTx(ethTx)

		txBuilder := txConfig.NewTxBuilder()
		if err := txBuilder.SetMsgs(msg); err != nil {
			return fmt.Errorf("failed to set msg in tx builder: %w", err)
		}

		txBytes, err := txConfig.TxEncoder()(txBuilder.GetTx())
		if err != nil {
			return fmt.Errorf("failed to encode transaction: %w", err)
		}

		res, err := clientCtx.BroadcastTxSync(txBytes)
		if err != nil {
			return fmt.Errorf("failed to broadcast transaction %s: %w", ethTx.Hash().Hex(), err)
		}
		if res.Code != 0 {
			return fmt.Errorf("transaction %s rejected by mempool: code=%d, log=%s", ethTx.Hash().Hex(), res.Code, res.RawLog)
		}
	}
	return nil
}
