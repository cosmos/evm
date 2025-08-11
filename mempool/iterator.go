package mempool

import (
	"math/big"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm/mempool/miner"
	"github.com/cosmos/evm/mempool/txpool"
	msgtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

var _ mempool.Iterator = &EVMMempoolIterator{}

// EVMMempoolIterator provides a unified iterator over both EVM and Cosmos transactions in the mempool.
// It implements priority-based transaction selection, choosing between EVM and Cosmos transactions
// based on their fee values. The iterator maintains state to track transaction types and ensures
// proper sequencing during block building.
type EVMMempoolIterator struct {
	/** Mempool Iterators **/
	evmIterator    *miner.TransactionsByPriceAndNonce
	cosmosIterator mempool.Iterator

	/** Utils **/
	txConfig client.TxConfig

	/** Chain Params **/
	bondDenom string
	chainID   *big.Int
}

// NewEVMMempoolIterator creates a new unified iterator over EVM and Cosmos transactions.
// It combines iterators from both transaction pools and selects transactions based on fee priority.
// Returns nil if both iterators are empty or nil. The bondDenom parameter specifies the native
// token denomination for fee comparisons, and chainId is used for EVM transaction conversion.
func NewEVMMempoolIterator(evmIterator *miner.TransactionsByPriceAndNonce, cosmosIterator mempool.Iterator, txConfig client.TxConfig, bondDenom string, chainID *big.Int) mempool.Iterator {
	// Check if we have any transactions at all
	hasEVM := evmIterator != nil && !evmIterator.Empty()
	hasCosmos := cosmosIterator != nil && cosmosIterator.Tx() != nil

	if !hasEVM && !hasCosmos {
		return nil
	}

	return &EVMMempoolIterator{
		evmIterator:    evmIterator,
		cosmosIterator: cosmosIterator,
		txConfig:       txConfig,
		bondDenom:      bondDenom,
		chainID:        chainID,
	}
}

// Next advances the iterator to the next transaction and returns the updated iterator.
// It determines which iterator (EVM or Cosmos) provided the current transaction and advances
// that iterator accordingly. Returns nil when no more transactions are available.
func (i *EVMMempoolIterator) Next() mempool.Iterator {
	// Get next transactions on both iterators to determine which iterator to advance
	nextEVMTx, _ := i.getNextEVMTx()
	nextCosmosTx, _ := i.getNextCosmosTx()

	// If no transactions available, we're done
	if nextEVMTx == nil && nextCosmosTx == nil {
		return nil
	}

	// Advance the iterator that provided the current transaction
	i.advanceCurrentIterator()

	// Check if we still have transactions after advancing
	if !i.hasMoreTransactions() {
		return nil
	}

	return i
}

// Tx returns the current transaction from the iterator.
// It selects between EVM and Cosmos transactions based on fee priority
// and converts EVM transactions to SDK format.
func (i *EVMMempoolIterator) Tx() sdk.Tx {
	// Get current transactions from both iterators
	nextEVMTx, _ := i.getNextEVMTx()
	nextCosmosTx, _ := i.getNextCosmosTx()

	// Return the preferred transaction based on fee priority
	return i.getPreferredTransaction(nextEVMTx, nextCosmosTx)
}

// =============================================================================
// UTILITY FUNCTIONS
// =============================================================================

// shouldUseEVM determines which transaction type to prioritize based on fee comparison.
// Returns true if the EVM transaction should be selected, false if Cosmos transaction should be used.
// EVM transactions will be prioritized in the following conditions:
// 1. Cosmos mempool has no transactions
// 2. EVM mempool has no transactions (fallback to Cosmos)
// 3. Cosmos transaction has no fee information
// 4. Cosmos transaction fee denomination doesn't match bond denom
// 5. Cosmos transaction fee is lower than the EVM transaction fee
// 6. Cosmos transaction fee overflows when converted to uint256
func (i *EVMMempoolIterator) shouldUseEVM() bool {
	// Get next transactions from both iterators
	nextEVMTx, evmFee := i.getNextEVMTx()
	nextCosmosTx, cosmosFee := i.getNextCosmosTx()

	// Handle cases where only one type is available
	if nextEVMTx == nil {
		return false // Use Cosmos when no EVM transaction available
	}
	if nextCosmosTx == nil {
		return true // Use EVM when no Cosmos transaction available
	}

	// Both have transactions - compare fees
	// cosmosFee can never be nil, but can be zero if no valid fee found
	if cosmosFee.IsZero() {
		return true // Use EVM if Cosmos transaction has no valid fee
	}

	// Compare fees - prefer EVM unless Cosmos has higher fee
	return !cosmosFee.Gt(evmFee)
}

// getNextEVMTx retrieves the next EVM transaction and its fee
func (i *EVMMempoolIterator) getNextEVMTx() (*txpool.LazyTransaction, *uint256.Int) {
	if i.evmIterator == nil {
		return nil, nil
	}
	return i.evmIterator.Peek()
}

// getNextCosmosTx retrieves the next Cosmos transaction and its fee
func (i *EVMMempoolIterator) getNextCosmosTx() (sdk.Tx, *uint256.Int) {
	if i.cosmosIterator == nil {
		return nil, nil
	}

	tx := i.cosmosIterator.Tx()
	if tx == nil {
		return nil, nil
	}

	// Extract fee from the transaction
	cosmosFee := i.extractCosmosFee(tx)
	if cosmosFee == nil {
		return tx, uint256.NewInt(0) // Return zero fee if no valid fee found
	}

	// Convert fee to uint256
	cosmosAmount, overflow := uint256.FromBig(cosmosFee.Amount.BigInt())
	if overflow {
		return tx, uint256.NewInt(0) // Return zero fee if overflow
	}

	return tx, cosmosAmount
}

// getPreferredTransaction returns the preferred transaction based on fee priority.
// Takes both transaction types as input and returns the preferred one, or nil if neither is available.
func (i *EVMMempoolIterator) getPreferredTransaction(nextEVMTx *txpool.LazyTransaction, nextCosmosTx sdk.Tx) sdk.Tx {
	// If no transactions available, return nil
	if nextEVMTx == nil && nextCosmosTx == nil {
		return nil
	}

	// Determine which transaction type to prioritize based on fee comparison
	useEVM := i.shouldUseEVM()

	if useEVM {
		// Prefer EVM transaction if available and convertible
		if nextEVMTx != nil {
			if evmTx := i.convertEVMToSDKTx(nextEVMTx); evmTx != nil {
				return evmTx
			}
		}
		// Fall back to Cosmos if EVM is not available or conversion fails
		return nextCosmosTx
	}

	// Prefer Cosmos transaction
	return nextCosmosTx
}

// advanceCurrentIterator advances the appropriate iterator based on which transaction was used
func (i *EVMMempoolIterator) advanceCurrentIterator() {
	useEVM := i.shouldUseEVM()

	if useEVM {
		// We used EVM transaction, advance EVM iterator
		// NOTE: EVM transactions are automatically removed by the maintenance loop in the txpool
		// so we shift instead of popping
		if i.evmIterator != nil {
			i.evmIterator.Shift()
		}
	} else {
		// We used Cosmos transaction (or EVM failed), advance Cosmos iterator
		if i.cosmosIterator != nil {
			i.cosmosIterator = i.cosmosIterator.Next()
		}
	}
}

// extractCosmosFee extracts the fee in bond denomination from a Cosmos transaction
func (i *EVMMempoolIterator) extractCosmosFee(tx sdk.Tx) *sdk.Coin {
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return nil // Transaction doesn't implement FeeTx interface
	}

	fees := feeTx.GetFee()
	for _, coin := range fees {
		if coin.Denom == i.bondDenom {
			return &coin
		}
	}
	return nil // No fee in bond denomination
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
		return nil
	}
	msgEthereumTx := &msgtypes.MsgEthereumTx{}
	if err := msgEthereumTx.FromSignedEthereumTx(nextEVMTx.Tx, ethtypes.LatestSignerForChainID(i.chainID)); err != nil {
		return nil // Return nil for invalid tx instead of panicking
	}
	cosmosTx, err := msgEthereumTx.BuildTx(i.txConfig.NewTxBuilder(), i.bondDenom)
	if err != nil {
		return nil
	}
	return cosmosTx
}
