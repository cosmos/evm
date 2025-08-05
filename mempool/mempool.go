package mempool

import (
	"context"
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	"errors"
	"fmt"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/cosmos/evm/mempool/miner"
	"github.com/cosmos/evm/mempool/txpool"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"sync"
)

var _ mempool.ExtMempool = &EVMMempool{}

type (
	// EVMMempool is a unified mempool that manages both EVM and Cosmos SDK transactions.
	// It provides a single interface for transaction insertion, selection, and removal while
	// maintaining separate pools for EVM and Cosmos transactions. The mempool handles
	// fee-based transaction prioritization and manages nonce sequencing for EVM transactions.
	EVMMempool struct {
		/** Keepers **/
		vmKeeper VMKeeperI

		/** Mempools **/
		txPool       *txpool.TxPool
		legacyTxPool *legacypool.LegacyPool
		cosmosPool   mempool.ExtMempool

		/** Utils **/
		txConfig   client.TxConfig
		blockchain *Blockchain
		bondDenom  string
		evmDenom   string

		/** Verification **/
		verifyTxFn func(tx sdk.Tx) ([]byte, error)

		/** Concurrency **/
		mtx sync.Mutex
	}
)

// EVMMempoolConfig contains configuration options for creating an EVMMempool.
// It allows customization of the underlying mempools, verification functions,
// and broadcasting functions used by the mempool.
type EVMMempoolConfig struct {
	TxPool        *txpool.TxPool
	CosmosPool    mempool.ExtMempool
	VerifyTxFn    func(tx sdk.Tx) ([]byte, error)
	BroadCastTxFn func(txs []*ethtypes.Transaction) error
}

// NewEVMMempool creates a new unified mempool for EVM and Cosmos transactions.
// It initializes both EVM and Cosmos transaction pools, sets up blockchain interfaces,
// and configures fee-based prioritization. The config parameter allows customization
// of pools and verification functions, with sensible defaults created if not provided.
func NewEVMMempool(ctx func(height int64, prove bool) (sdk.Context, error), vmKeeper VMKeeperI, feeMarketKeeper FeeMarketKeeperI, txConfig client.TxConfig, clientCtx client.Context, config *EVMMempoolConfig) *EVMMempool {
	var txPool *txpool.TxPool
	var cosmosPool mempool.ExtMempool
	var verifyTxFn func(tx sdk.Tx) ([]byte, error)

	bondDenom := evmtypes.GetEVMCoinDenom()
	evmDenom := types.ExtendedCoinDenom()

	if config == nil {
		panic("config must not be nil")
	}

	txPool = config.TxPool
	cosmosPool = config.CosmosPool
	verifyTxFn = config.VerifyTxFn

	var blockchain *Blockchain

	// Default txPool
	if txPool == nil {
		blockchain = NewBlockchain(ctx, vmKeeper, feeMarketKeeper)
		legacyPool := legacypool.New(legacypool.DefaultConfig, blockchain)

		// Set up broadcast function using clientCtx
		if config.BroadCastTxFn != nil {
			legacyPool.BroadCastTxFn = config.BroadCastTxFn
		} else {
			// Create default broadcast function using clientCtx.
			// The EVM mempool will broadcast transactions when it promotes them
			// from queued into pending, noting their readiness to be executed.
			legacyPool.BroadCastTxFn = func(txs []*ethtypes.Transaction) error {
				return broadcastEVMTransactions(clientCtx, txConfig, txs)
			}
		}

		txPoolInit, err := txpool.New(uint64(0), blockchain, []txpool.SubPool{legacyPool})
		if err != nil {
			panic(err)
		}
		txPool = txPoolInit
	}

	// Default Cosmos Mempool
	if cosmosPool == nil {
		priorityConfig := sdkmempool.PriorityNonceMempoolConfig[math.Int]{}
		priorityConfig.TxPriority = sdkmempool.TxPriority[math.Int]{
			GetTxPriority: func(goCtx context.Context, tx sdk.Tx) math.Int {
				cosmosTxFee, ok := tx.(sdk.FeeTx)
				if !ok {
					return math.ZeroInt()
				}
				found, coin := cosmosTxFee.GetFee().Find(bondDenom)
				if !found {
					return math.ZeroInt()
				}
				return coin.Amount
			},
			Compare: func(a, b math.Int) int {
				return a.BigInt().Cmp(b.BigInt())
			},
			MinValue: math.ZeroInt(),
		}
		cosmosPool = sdkmempool.NewPriorityMempool(priorityConfig)
	}

	if len(txPool.Subpools) != 1 {
		panic("tx pool should contain one subpool")
	}
	if _, ok := txPool.Subpools[0].(*legacypool.LegacyPool); !ok {
		panic("tx pool should contain only legacypool")
	}

	return &EVMMempool{
		vmKeeper:     vmKeeper,
		txPool:       txPool,
		legacyTxPool: txPool.Subpools[0].(*legacypool.LegacyPool),
		cosmosPool:   cosmosPool,
		txConfig:     txConfig,
		blockchain:   blockchain,
		bondDenom:    bondDenom,
		evmDenom:     evmDenom,
		verifyTxFn:   verifyTxFn,
	}
}

// GetBlockchain returns the blockchain interface used for chain head event notifications.
// This is primarily used to notify the mempool when new blocks are finalized.
func (m *EVMMempool) GetBlockchain() *Blockchain {
	return m.blockchain
}

// GetTxPool returns the underlying EVM txpool.
// This provides direct access to the EVM-specific transaction management functionality.
func (m *EVMMempool) GetTxPool() *txpool.TxPool {
	return m.txPool
}

// Insert adds a transaction to the appropriate mempool (EVM or Cosmos).
// EVM transactions are routed to the EVM transaction pool, while all other
// transactions are inserted into the Cosmos mempool. The method assumes
// transactions have already passed CheckTx validation.
func (m *EVMMempool) Insert(goCtx context.Context, tx sdk.Tx) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	ctx := sdk.UnwrapSDKContext(goCtx)
	if ctx.BlockHeight() < 2 {
		return errorsmod.Wrap(sdkerrors.ErrInvalidHeight, "Mempool is not ready. Please wait for block 1 to finalize.")
	}

	ethMsg, err := m.getEVMMessage(tx)
	if err == nil {
		// Insert into EVM pool
		ethTxs := []*ethtypes.Transaction{ethMsg.AsTransaction()}
		errs := m.txPool.Add(ethTxs, true)
		if len(errs) > 0 && errs[0] != nil {
			return errs[0]
		}
		return nil
	}

	// Insert into cosmos pool for non-EVM transactions
	return m.cosmosPool.Insert(goCtx, tx)
}

// InsertInvalidNonce handles transactions that failed with nonce gap errors.
// It attempts to insert EVM transactions into the pool as non-local transactions,
// allowing them to be queued for future execution when the nonce gap is filled.
// Non-EVM transactions are discarded as regular Cosmos flows do not support nonce gaps.
func (m *EVMMempool) InsertInvalidNonce(txBytes []byte) error {
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
	errs := m.txPool.Add(ethTxs, false) // TODO: proper sync parameters
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
func (m *EVMMempool) Select(goCtx context.Context, i [][]byte) mempool.Iterator {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	evmIterator, cosmosIterator := m.getIterators(goCtx, i)

	combinedIterator := NewEVMMempoolIterator(evmIterator, cosmosIterator, m.txConfig, m.bondDenom, m.blockchain.Config().ChainID)

	return combinedIterator
}

// CountTx returns the total number of transactions in both EVM and Cosmos pools.
// This provides a combined count across all mempool types.
func (m *EVMMempool) CountTx() int {
	pending, _ := m.txPool.Stats()
	return m.cosmosPool.CountTx() + pending
}

// Remove removes a transaction from the appropriate mempool.
// For EVM transactions, removal is typically handled automatically by the pool
// based on nonce progression. Cosmos transactions are removed from the Cosmos pool.
func (m *EVMMempool) Remove(tx sdk.Tx) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	msg, err := m.getEVMMessage(tx)
	if err == nil {
		// Comet will attempt to remove transactions from the mempool after completing successfully.
		// We should not do this with EVM transactions because removing them causes the subsequent ones to
		// be dequeued as temporarily invalid, only to be requeued a block later.
		// The EVM mempool handles removal based on account nonce automatically.
		if m.shouldRemoveFromEVMPool(tx) {
			m.legacyTxPool.RemoveTx(common.HexToHash(msg.Hash), false, true)
		}
		return nil
	}

	if errors.Is(err, ErrNoMessages) {
		return err
	}

	return m.cosmosPool.Remove(tx)
}

// shouldRemoveFromEVMPool determines whether an EVM transaction should be manually removed.
// It uses the verification function to check if the transaction failed for reasons
// other than nonce gaps or successful execution, in which case manual removal is needed.
func (m *EVMMempool) shouldRemoveFromEVMPool(tx sdk.Tx) bool {
	if m.verifyTxFn == nil {
		return false
	}

	// This is an inefficiency that could be optimized by getting more information from the removal function.
	// Currently, we remove transactions on completion and on recheckTx errors. However, we do not know why the removal
	// happened, so we need to reverify. If it was a successful transaction or a sequence error, we let the mempool handle the cleaning.
	// If it was any other Cosmos or antehandler related issue, then we remove it.
	_, err := m.verifyTxFn(tx)
	if err == nil {
		return false
	}

	if errors.Is(err, ErrNonceGap) || errors.Is(err, sdkerrors.ErrInvalidSequence) {
		return false
	}

	return true
}

// SelectBy iterates through transactions until the provided filter function returns false.
// It uses the same unified iterator as Select but allows early termination based on
// custom criteria defined by the filter function.
func (m *EVMMempool) SelectBy(goCtx context.Context, i [][]byte, f func(sdk.Tx) bool) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	evmIterator, cosmosIterator := m.getIterators(goCtx, i)

	var combinedIterator = NewEVMMempoolIterator(evmIterator, cosmosIterator, m.txConfig, m.bondDenom, m.blockchain.Config().ChainID)

	for combinedIterator != nil && f(combinedIterator.Tx()) {
		combinedIterator = combinedIterator.Next()
	}
}

// getEVMMessage validates that the transaction contains exactly one message and returns it if it's an EVM message.
// Returns an error if the transaction has no messages, multiple messages, or the single message is not an EVM transaction.
func (m *EVMMempool) getEVMMessage(tx sdk.Tx) (*evmtypes.MsgEthereumTx, error) {
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
func (m *EVMMempool) getIterators(goCtx context.Context, i [][]byte) (*miner.TransactionsByPriceAndNonce, sdkmempool.Iterator) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	baseFee := m.vmKeeper.GetBaseFee(ctx)
	var baseFeeUint *uint256.Int
	if baseFee != nil {
		baseFeeUint = uint256.MustFromBig(baseFee)
	}

	pendingFilter := txpool.PendingFilter{
		MinTip:       nil,
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
		if err := msg.FromEthereumTx(ethTx); err != nil {
			return fmt.Errorf("failed to convert EVM tx to Cosmos msg: %w", err)
		}

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
