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

// shouldUseEVM determines which transaction type to prioritize based on fee comparison.
// Returns true if the EVM transaction should be selected, false if Cosmos transaction should be used.
// EVM transactions are preferred when Cosmos transactions lack fees or use non-bondDenom denominations.
func (i *EVMMempoolIterator) shouldUseEVM() bool {
	var nextEVMTx *txpool.LazyTransaction
	var evmFee *uint256.Int
	if i.evmIterator != nil {
		nextEVMTx, evmFee = i.evmIterator.Peek()
	}

	var nextCosmosTx sdk.Tx
	if i.cosmosIterator != nil {
		nextCosmosTx = i.cosmosIterator.Tx()
	}

	// If only one type available
	if nextEVMTx == nil {
		return false // Use Cosmos
	}
	if nextCosmosTx == nil {
		return true // Use EVM
	}

	// Both have transactions - compare fees
	cosmosTxFee, ok := nextCosmosTx.(sdk.FeeTx)
	if !ok {
		return true // Use EVM if Cosmos tx has no fee
	}

	cosmosFees := cosmosTxFee.GetFee()
	var cosmosTxEVMDenomFee *sdk.Coin
	for _, coin := range cosmosFees {
		if coin.Denom == i.bondDenom {
			cosmosTxEVMDenomFee = &coin
			break
		}
	}

	if cosmosTxEVMDenomFee == nil {
		return true // Use EVM if Cosmos tx has wrong denomination
	}

	cosmosTxAmount, overflow := uint256.FromBig(cosmosTxEVMDenomFee.Amount.BigInt())
	if overflow || !cosmosTxAmount.Gt(evmFee) {
		return true // Use EVM if Cosmos fee is not higher
	}

	return false // Use Cosmos if it has higher fee
}

// Next advances the iterator to the next transaction and returns the updated iterator.
// It determines which iterator (EVM or Cosmos) provided the current transaction and advances
// that iterator accordingly. Returns nil when no more transactions are available.
func (i *EVMMempoolIterator) Next() mempool.Iterator {
	// Determine what transaction we just returned and advance the correct iterator
	var nextEVMTx *txpool.LazyTransaction
	if i.evmIterator != nil {
		nextEVMTx, _ = i.evmIterator.Peek()
	}

	var nextCosmosTx sdk.Tx
	if i.cosmosIterator != nil {
		nextCosmosTx = i.cosmosIterator.Tx()
	}

	// If no transactions available, we're done
	if nextEVMTx == nil && nextCosmosTx == nil {
		return nil
	}

	// Advance the iterator that provided the current transaction
	// NOTE: We don't modify the EVM pool during iteration - transactions
	// are automatically removed by the maintenance loop in the txpool.
	if i.shouldUseEVM() {
		if nextEVMTx != nil {
			// We used EVM transaction, advance EVM iterator
			if i.evmIterator != nil {
				i.evmIterator.Shift()
			}
		} else {
			// No EVM tx, we used cosmos, advance cosmos
			if i.cosmosIterator != nil {
				i.cosmosIterator = i.cosmosIterator.Next()
			}
		}
	} else {
		// We used cosmos transaction, advance cosmos iterator
		if i.cosmosIterator != nil {
			i.cosmosIterator = i.cosmosIterator.Next()
		}
	}

	// Check if we still have transactions after advancing
	var hasMoreEVM, hasMoreCosmos bool
	if i.evmIterator != nil {
		nextEVMTx, _ = i.evmIterator.Peek()
		hasMoreEVM = nextEVMTx != nil
	}
	if i.cosmosIterator != nil {
		hasMoreCosmos = i.cosmosIterator.Tx() != nil
	}

	if !hasMoreEVM && !hasMoreCosmos {
		return nil
	}

	return i
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

// Tx returns the current transaction from the iterator.
// It selects between EVM and Cosmos transactions based on fee priority
// and converts EVM transactions to SDK format.
func (i *EVMMempoolIterator) Tx() sdk.Tx {
	// Get current transactions from both iterators
	var nextEVMTx *txpool.LazyTransaction
	if i.evmIterator != nil {
		nextEVMTx, _ = i.evmIterator.Peek()
	}

	var nextCosmosTx sdk.Tx
	if i.cosmosIterator != nil {
		nextCosmosTx = i.cosmosIterator.Tx()
	}

	// If no transactions available, this shouldn't happen if iterator was created properly
	if nextEVMTx == nil && nextCosmosTx == nil {
		return nil
	}

	// Return the appropriate transaction based on priority
	if i.shouldUseEVM() {
		if nextEVMTx != nil {
			evmTx := i.convertEVMToSDKTx(nextEVMTx)
			if evmTx != nil {
				return evmTx
			}
		}
		// Fall back to cosmos if EVM fails
		return nextCosmosTx
	}
	return nextCosmosTx
}
