package mempool

import (
	"fmt"
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

// EVMMempoolIterator provides a unified iterator over both EVM and Cosmos transactions in the mempool.
// It implements priority-based transaction selection, choosing between EVM and Cosmos transactions
// based on their fee values. The iterator maintains state to track transaction types and ensures
// proper sequencing during block building.
type EVMMempoolIterator struct {
	/** Mempool Iterators **/
	evmIterator    *miner.TransactionsByPriceAndNonce
	cosmosIterator mempool.Iterator

	/** Cached current state - recomputed only when iterator advances **/
	currentTx     sdk.Tx // The resolved preferred tx for the current position
	advanceEVM    bool   // Whether the current preferred tx came from the EVM iterator
	advanceCosmos bool

	/** Cached chain params - computed once at construction **/
	baseFee   *uint256.Int    // Base fee, cached from blockchain header
	ethSigner ethtypes.Signer // Cached EVM signer

	/** Utils **/
	logger   log.Logger
	txConfig client.TxConfig

	/** Chain Params **/
	bondDenom string
	chainID   *big.Int
}

// NewEVMMempoolIterator creates a new unified iterator over EVM and Cosmos transactions.
// It combines iterators from both transaction pools and selects transactions based on fee priority.
// Returns nil if both iterators are empty or nil. The bondDenom parameter specifies the native
// token denomination for fee comparisons, and chainId is used for EVM transaction conversion.
func NewEVMMempoolIterator(
	evmIterator *miner.TransactionsByPriceAndNonce,
	cosmosIterator mempool.Iterator,
	logger log.Logger,
	txConfig client.TxConfig,
	bondDenom string,
	chainID *big.Int,
	blockchain *Blockchain,
) mempool.Iterator {
	hasEVM := evmIterator != nil && !evmIterator.Empty()
	hasCosmos := cosmosIterator != nil && cosmosIterator.Tx() != nil

	if !hasEVM && !hasCosmos {
		return nil
	}

	iter := &EVMMempoolIterator{
		evmIterator:    evmIterator,
		cosmosIterator: cosmosIterator,
		logger:         logger,
		txConfig:       txConfig,
		bondDenom:      bondDenom,
		chainID:        chainID,
		ethSigner:      ethtypes.LatestSignerForChainID(chainID),
		baseFee:        currentBaseFee(blockchain),
	}

	// Eagerly resolve the first preferred transaction
	iter.resolveCurrentTx()

	if iter.currentTx == nil {
		return nil
	}

	return iter
}

// Next advances the iterator to the next transaction and returns the updated iterator.
// It determines which iterator (EVM or Cosmos) provided the current transaction and advances
// that iterator accordingly. Returns nil when no more transactions are available.
func (i *EVMMempoolIterator) Next() mempool.Iterator {
	// advance iterators
	switch {
	case i.advanceEVM:
		i.advanceEVM = false
		if i.evmIterator != nil {
			i.evmIterator.Shift()
		}
	case i.advanceCosmos:
		i.advanceCosmos = false
		if i.cosmosIterator != nil {
			i.cosmosIterator = i.cosmosIterator.Next()
		}
	}

	// resolve the next preferred transaction
	i.resolveCurrentTx()

	if i.currentTx == nil {
		return nil
	}

	return i
}

// Tx returns the current transaction from the iterator.
func (i *EVMMempoolIterator) Tx() sdk.Tx {
	return i.currentTx
}

// resolveCurrentTx determines the preferred transaction between the EVM and Cosmos
// iterators and caches it. This is called once at construction and once after each
// advance, eliminating all redundant fee calculations and iterator peeks.
func (i *EVMMempoolIterator) resolveCurrentTx() {
	evmTx, evmFee := i.peekEVM()
	cosmosTx, cosmosFee := i.peekCosmos()

	if evmTx == nil && cosmosTx == nil {
		i.advanceEVM, i.advanceCosmos = false, false
		i.currentTx = nil
		return
	}

	useEVM := i.compareFeePriority(evmTx, evmFee, cosmosTx, cosmosFee)

	if useEVM {
		i.advanceEVM = true
		sdkTx, err := i.convertEVMToSDKTx(evmTx)
		if err == nil {
			i.currentTx = sdkTx
			return
		}
		i.logger.Error("EVM transaction conversion failed, falling back to Cosmos transaction", "tx_hash", evmTx.Hash, "err", err)

		// Even if the conversion to cosmos tx failed, we still want advanceEVM
		// to be true, so that this invalid tx gets skipped over when we
		// advance iterators, i.e. if we are here both advanceCosmos and
		// advanceEVM will be true
	}

	i.advanceCosmos = true
	i.currentTx = cosmosTx
}

// compareFeePriority determines which transaction type to prioritize based on fee comparison.
// Returns true if the EVM transaction should be selected.
func (i *EVMMempoolIterator) compareFeePriority(evmTx *txpool.LazyTransaction, evmFee *uint256.Int, cosmosTx sdk.Tx, cosmosFee *uint256.Int) bool {
	if evmTx == nil {
		return false
	}
	if cosmosTx == nil {
		return true
	}

	// Both have transactions - compare fees
	if cosmosFee.IsZero() {
		return true // Use EVM if Cosmos transaction has no valid fee
	}

	// Prefer EVM unless Cosmos has strictly higher fee
	return !cosmosFee.Gt(evmFee)
}

// peekEVM retrieves the next EVM transaction and its fee effective gas tip
// without advancing.
func (i *EVMMempoolIterator) peekEVM() (*txpool.LazyTransaction, *uint256.Int) {
	if i.evmIterator == nil {
		return nil, nil
	}
	return i.evmIterator.Peek()
}

// peekCosmos retrieves the next Cosmos transaction and its effective gas tip
// without advancing.
func (i *EVMMempoolIterator) peekCosmos() (sdk.Tx, *uint256.Int) {
	if i.cosmosIterator == nil {
		return nil, nil
	}

	tx := i.cosmosIterator.Tx()
	if tx == nil {
		return nil, nil
	}

	tip := i.extractCosmosEffectiveTip(tx)
	if tip == nil {
		return tx, uint256.NewInt(0)
	}

	return tx, tip
}

// extractCosmosEffectiveTip extracts the effective gas tip from a Cosmos transaction
// This aligns with EVM transaction prioritization by calculating: gas_price - base_fee
func (i *EVMMempoolIterator) extractCosmosEffectiveTip(tx sdk.Tx) *uint256.Int {
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return nil
	}

	bondDenomFeeAmount := math.ZeroInt()
	fees := feeTx.GetFee()
	for _, coin := range fees {
		if coin.Denom == i.bondDenom {
			bondDenomFeeAmount = coin.Amount
		}
	}

	// Calculate gas price: fee_amount / gas_limit
	gasPrice, overflow := uint256.FromBig(bondDenomFeeAmount.Quo(math.NewIntFromUint64(feeTx.GetGas())).BigInt())
	if overflow {
		return nil
	}

	// Subtract base fee if available
	if i.baseFee == nil {
		return gasPrice
	}

	if gasPrice.Cmp(i.baseFee) < 0 {
		return uint256.NewInt(0)
	}

	return new(uint256.Int).Sub(gasPrice, i.baseFee)
}

// convertEVMToSDKTx converts an Ethereum transaction to a Cosmos SDK transaction.
// It wraps the EVM transaction in a MsgEthereumTx and builds a proper SDK transaction
// using the configured transaction builder and bond denomination for fees.
func (i *EVMMempoolIterator) convertEVMToSDKTx(nextEVMTx *txpool.LazyTransaction) (sdk.Tx, error) {
	if nextEVMTx == nil {
		return nil, fmt.Errorf("next evm tx is nil")
	}

	var msgEthereumTx msgtypes.MsgEthereumTx
	if err := msgEthereumTx.FromSignedEthereumTx(nextEVMTx.Tx, i.ethSigner); err != nil {
		return nil, fmt.Errorf("converting signed evm transaction: %w", err)
	}

	cosmosTx, err := msgEthereumTx.BuildTx(i.txConfig.NewTxBuilder(), i.bondDenom)
	if err != nil {
		return nil, fmt.Errorf("building cosmos tx from evm tx: %w", err)
	}

	return cosmosTx, nil
}

// currentBaseFee gets the current baseFee from the Blockchain based on the
// latest block.
func currentBaseFee(blockchain *Blockchain) *uint256.Int {
	if blockchain == nil {
		return nil
	}

	header := blockchain.CurrentBlock()
	if header == nil || header.BaseFee == nil {
		return nil
	}

	baseFeeUint, overflow := uint256.FromBig(header.BaseFee)
	if overflow {
		return nil
	}

	return baseFeeUint
}
