package mempool

import (
	"math/big"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm/mempool/miner"
	"github.com/cosmos/evm/mempool/txpool"
	msgtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

var _ mempool.Iterator = &EVMMempoolIterator{}

type evmTxIterator interface {
	Peek() (*txpool.LazyTransaction, *uint256.Int)
	Shift()
	Empty() bool
}

type txSource uint8

const (
	txSourceUnknown txSource = iota
	txSourceEVM
	txSourceCosmos
)

// EVMMempoolIterator provides a unified iterator over both EVM and Cosmos transactions in the mempool.
// It implements priority-based transaction selection, choosing between EVM and Cosmos transactions
// based on their fee values. The iterator maintains state to track transaction types and ensures
// proper sequencing during block building.
type EVMMempoolIterator struct {
	/** Mempool Iterators **/
	evmIterator    evmTxIterator
	cosmosIterator mempool.Iterator
	lastSource     txSource

	/** Utils **/
	logger   log.Logger
	txConfig client.TxConfig

	/** Chain Params **/
	bondDenom string
	chainID   *big.Int

	/** Blockchain Access **/
	blockchain *Blockchain
}

// NewEVMMempoolIterator creates a new unified iterator over EVM and Cosmos transactions.
// It combines iterators from both transaction pools and selects transactions based on fee priority.
// Returns nil if both iterators are empty or nil. The bondDenom parameter specifies the native
// token denomination for fee comparisons, and chainId is used for EVM transaction conversion.
func NewEVMMempoolIterator(evmIterator *miner.TransactionsByPriceAndNonce, cosmosIterator mempool.Iterator, logger log.Logger, txConfig client.TxConfig, bondDenom string, chainID *big.Int, blockchain *Blockchain) mempool.Iterator {
	// Check if we have any transactions at all
	hasEVM := evmIterator != nil && !evmIterator.Empty()
	hasCosmos := cosmosIterator != nil && cosmosIterator.Tx() != nil

	// Add the iterator name to the logger
	logger = logger.With(log.ModuleKey, "EVMMempoolIterator")

	if !hasEVM && !hasCosmos {
		logger.Debug("no transactions available in either mempool")
		return nil
	}

	return &EVMMempoolIterator{
		evmIterator:    evmIterator,
		cosmosIterator: cosmosIterator,
		logger:         logger,
		txConfig:       txConfig,
		bondDenom:      bondDenom,
		chainID:        chainID,
		blockchain:     blockchain,
		lastSource:     txSourceUnknown,
	}
}

// Next advances the iterator to the next transaction and returns the updated iterator.
// It determines which iterator (EVM or Cosmos) provided the current transaction and advances
// that iterator accordingly. Returns nil when no more transactions are available.
func (i *EVMMempoolIterator) Next() mempool.Iterator {
	if !i.hasMoreTransactions() {
		i.logger.Debug("no more transactions available, ending iteration")
		return nil
	}

	// If Tx() wasn't called before Next(), pick a source now so that
	// advancing is consistent with what Tx() would return.
	if i.lastSource == txSourceUnknown {
		_ = i.Tx()
	}

	i.advanceCurrentIterator(i.lastSource)
	i.lastSource = txSourceUnknown

	// Check if we still have transactions after advancing
	if !i.hasMoreTransactions() {
		i.logger.Debug("no more transactions after advancing, ending iteration")
		return nil
	}

	return i
}

// Tx returns the current transaction from the iterator.
// It selects between EVM and Cosmos transactions based on fee priority
// and converts EVM transactions to SDK format.
func (i *EVMMempoolIterator) Tx() sdk.Tx {
	// Get current transactions (and fees) from both iterators
	nextEVMTx, evmFee := i.getNextEVMTx()
	nextCosmosTx, cosmosFee := i.getNextCosmosTx()

	i.logger.Debug("getting current transaction", "has_evm", nextEVMTx != nil, "has_cosmos", nextCosmosTx != nil)

	tx, source := i.getPreferredTransaction(nextEVMTx, evmFee, nextCosmosTx, cosmosFee)
	i.lastSource = source

	if tx == nil {
		i.logger.Debug("no preferred transaction available")
	} else {
		i.logger.Debug("returning preferred transaction")
	}

	return tx
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

// getNextEVMTx retrieves the next EVM transaction and its fee
func (i *EVMMempoolIterator) getNextEVMTx() (*txpool.LazyTransaction, *uint256.Int) {
	if i.evmIterator == nil {
		return nil, nil
	}
	return i.evmIterator.Peek()
}

// getNextCosmosTx retrieves the next Cosmos transaction and its effective gas tip
func (i *EVMMempoolIterator) getNextCosmosTx() (sdk.Tx, *uint256.Int) {
	if i.cosmosIterator == nil {
		return nil, nil
	}

	tx := i.cosmosIterator.Tx()
	if tx == nil {
		return nil, nil
	}

	// Extract effective gas tip from the transaction (gas price - base fee)
	cosmosEffectiveTip := i.extractCosmosEffectiveTip(tx)
	if cosmosEffectiveTip == nil {
		return tx, uint256.NewInt(0) // Return zero fee if no valid fee found
	}

	return tx, cosmosEffectiveTip
}

// getPreferredTransaction returns the preferred transaction based on fee priority.
//
// Rules:
// - If only one pool has a tx, pick that tx.
// - Treat a missing/invalid Cosmos effective tip as 0 and prefer EVM.
// - Otherwise prefer EVM unless Cosmos has a strictly higher effective tip (EVM wins ties).
//
// cosmosFee is the already-derived Cosmos effective tip; any issues while computing it (e.g. fee
// missing/invalid, denom mismatch, overflow) are represented as a zero/invalid tip upstream.
func (i *EVMMempoolIterator) getPreferredTransaction(
	nextEVMTx *txpool.LazyTransaction, evmFee *uint256.Int,
	nextCosmosTx sdk.Tx, cosmosFee *uint256.Int,
) (sdk.Tx, txSource) {
	// If no transactions available, return nil
	if nextEVMTx == nil && nextCosmosTx == nil {
		i.logger.Debug("no transactions available from either mempool")
		return nil, txSourceUnknown
	}

	if evmFee == nil {
		evmFee = uint256.NewInt(0)
	}
	if cosmosFee == nil {
		cosmosFee = uint256.NewInt(0)
	}

	// Decide which tx type to use based on fee comparison.
	useEVM := false
	switch {
	case nextEVMTx == nil:
		useEVM = false
	case nextCosmosTx == nil:
		useEVM = true
	case cosmosFee.IsZero():
		// No valid Cosmos fee/tip; prefer EVM.
		useEVM = true
	default:
		// Prefer EVM unless Cosmos has a higher effective tip.
		useEVM = !cosmosFee.Gt(evmFee)
	}

	if useEVM {
		i.logger.Debug("preferring EVM transaction based on fee comparison")
		// Prefer EVM transaction if available and convertible
		if nextEVMTx != nil {
			if evmTx := i.convertEVMToSDKTx(nextEVMTx); evmTx != nil {
				return evmTx, txSourceEVM
			}
		}
		// Fall back to Cosmos if EVM is not available or conversion fails
		i.logger.Debug("EVM transaction conversion failed, falling back to Cosmos transaction")
		if nextCosmosTx != nil {
			return nextCosmosTx, txSourceCosmos
		}
		return nil, txSourceEVM
	}

	// Prefer Cosmos transaction
	i.logger.Debug("preferring Cosmos transaction based on fee comparison")
	if nextCosmosTx != nil {
		return nextCosmosTx, txSourceCosmos
	}
	return nil, txSourceEVM
}

// advanceCurrentIterator advances the iterator that produced the transaction returned by Tx().
func (i *EVMMempoolIterator) advanceCurrentIterator(source txSource) {
	switch source {
	case txSourceEVM:
		i.logger.Debug("advancing EVM iterator")
		// NOTE: EVM transactions are automatically removed by the maintenance loop in the txpool
		// so we shift instead of popping.
		if i.evmIterator != nil {
			i.evmIterator.Shift()
		} else {
			i.logger.Error("EVM iterator is nil but advanceCurrentIterator selected EVM")
		}
	case txSourceCosmos:
		i.logger.Debug("advancing Cosmos iterator")
		if i.cosmosIterator != nil {
			i.cosmosIterator = i.cosmosIterator.Next()
		} else {
			i.logger.Error("Cosmos iterator is nil but advanceCurrentIterator selected Cosmos")
		}
	default:
		// If we can't determine a source, don't advance anything.
		i.logger.Debug("cannot advance: no source selected")
	}
}

// extractCosmosEffectiveTip extracts the effective gas tip from a Cosmos transaction
// This aligns with EVM transaction prioritization by calculating: gas_price - base_fee
func (i *EVMMempoolIterator) extractCosmosEffectiveTip(tx sdk.Tx) *uint256.Int {
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		i.logger.Debug("Cosmos transaction doesn't implement FeeTx interface")
		return nil // Transaction doesn't implement FeeTx interface
	}

	bondDenomFeeAmount := math.ZeroInt()
	fees := feeTx.GetFee()
	for _, coin := range fees {
		if coin.Denom == i.bondDenom {
			i.logger.Debug("found fee in bond denomination", "denom", coin.Denom, "amount", coin.Amount.String())
			bondDenomFeeAmount = coin.Amount
		}
	}

	// Calculate gas price: fee_amount / gas_limit
	gasPrice, overflow := uint256.FromBig(bondDenomFeeAmount.Quo(math.NewIntFromUint64(feeTx.GetGas())).BigInt())
	if overflow {
		i.logger.Debug("overflowed on gas price calculation")
		return nil
	}

	// Get current base fee from blockchain StateDB
	baseFee := i.getCurrentBaseFee()
	if baseFee == nil {
		// No base fee, return gas price as effective tip
		i.logger.Debug("no base fee available, using gas price as effective tip", "gas_price", gasPrice.String())
		return gasPrice
	}

	// Calculate effective tip: gas_price - base_fee
	if gasPrice.Cmp(baseFee) < 0 {
		// Gas price is lower than base fee, return zero effective tip
		i.logger.Debug("gas price lower than base fee, effective tip is zero", "gas_price", gasPrice.String(), "base_fee", baseFee.String())
		return uint256.NewInt(0)
	}

	effectiveTip := new(uint256.Int).Sub(gasPrice, baseFee)
	i.logger.Debug("calculated effective tip", "gas_price", gasPrice.String(), "base_fee", baseFee.String(), "effective_tip", effectiveTip.String())
	return effectiveTip
}

// getCurrentBaseFee retrieves the current base fee from the blockchain StateDB
func (i *EVMMempoolIterator) getCurrentBaseFee() *uint256.Int {
	if i.blockchain == nil {
		i.logger.Debug("blockchain not available, cannot get base fee")
		return nil
	}

	// Get the current block header to access the base fee
	header := i.blockchain.CurrentBlock()
	if header == nil {
		i.logger.Debug("failed to get current block header")
		return nil
	}

	// Get base fee from the header
	baseFee := header.BaseFee
	if baseFee == nil {
		i.logger.Debug("no base fee in current block header")
		return nil
	}

	// Convert to uint256
	baseFeeUint, overflow := uint256.FromBig(baseFee)
	if overflow {
		i.logger.Debug("base fee overflow when converting to uint256")
		return nil
	}

	i.logger.Debug("retrieved current base fee from blockchain", "base_fee", baseFeeUint.String())
	return baseFeeUint
}

// hasMoreTransactions checks if there are more transactions available in either iterator
func (i *EVMMempoolIterator) hasMoreTransactions() bool {
	nextEVMTx, _ := i.getNextEVMTx()
	nextCosmosTx, _ := i.getNextCosmosTx()
	return nextEVMTx != nil || nextCosmosTx != nil
}

// convertEVMToSDKTx converts an Ethereum transaction to a Cosmos SDK transaction.
// It wraps the EVM transaction in a MsgEthereumTx and builds a proper SDK transaction
// using the configured transaction builder and bond denomination for fees.
func (i *EVMMempoolIterator) convertEVMToSDKTx(nextEVMTx *txpool.LazyTransaction) sdk.Tx {
	if nextEVMTx == nil {
		i.logger.Debug("EVM transaction is nil, skipping conversion")
		return nil
	}

	msgEthereumTx := &msgtypes.MsgEthereumTx{}
	hash := nextEVMTx.Tx.Hash()
	if err := msgEthereumTx.FromSignedEthereumTx(nextEVMTx.Tx, ethtypes.LatestSignerForChainID(i.chainID)); err != nil {
		i.logger.Error("failed to convert signed Ethereum transaction", "error", err, "tx_hash", hash)
		return nil // Return nil for invalid tx instead of panicking
	}

	cosmosTx, err := msgEthereumTx.BuildTx(i.txConfig.NewTxBuilder(), i.bondDenom)
	if err != nil {
		i.logger.Error("failed to build Cosmos transaction from EVM transaction", "error", err, "tx_hash", hash)
		return nil
	}

	i.logger.Debug("successfully converted EVM transaction to Cosmos transaction", "tx_hash", hash)
	return cosmosTx
}
