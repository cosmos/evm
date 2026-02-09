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

// nextAction is the type of action that the iterator should take based on the
// current transaction selection
type nextAction int

const (
	// advance signals to the iterator that it should move the iterator
	// pointer to the next best tx
	advance nextAction = iota

	// skipAccount signals to the iterator that it should move the iterator
	// pointer past all txs for this account, to the next best tx that is not
	// for this account
	skipAccount

	// none signals that no action should be taken for this iterator
	none
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

	// currentTx is the preferred tx to return via Tx() based on the state of
	// both internal iterators
	currentTx sdk.Tx

	// nextEVMAction is the action that the evm iterator should take on the next
	// call to Next()
	nextEVMAction nextAction

	// nextCosmosAction is the action that the cosmos iterator should take on the next
	// call to Next()
	nextCosmosAction nextAction

	/** Utils **/
	logger   log.Logger
	txConfig client.TxConfig

	/** Chain Params **/
	bondDenom string
	chainID   *big.Int
	baseFee   *uint256.Int    // cached on iterator creation
	ethSigner ethtypes.Signer // cached on iterator creation
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
		evmIterator:      evmIterator,
		cosmosIterator:   cosmosIterator,
		nextEVMAction:    none,
		nextCosmosAction: none,
		logger:           logger,
		txConfig:         txConfig,
		bondDenom:        bondDenom,
		chainID:          chainID,
		ethSigner:        ethtypes.LatestSignerForChainID(chainID),
		baseFee:          currentBaseFee(blockchain),
	}

	// setup internal currentTx state
	iter.resolveCurrentTx()

	// if there is no currentTx, we have no txs to return
	if iter.currentTx == nil {
		return nil
	}

	return iter
}

// Next advances the iterator to the next transaction and returns the updated iterator.
// It determines which iterator (EVM or Cosmos) provided the current transaction and advances
// that iterator accordingly. Returns nil when no more transactions are available.
func (i *EVMMempoolIterator) Next() mempool.Iterator {
	// increment iterators forward based on action that was determined to be
	// taken previous call to Next()
	i.handleNextEVMAction()
	i.handleNextCosmosAction()

	// resolve the next preferred transaction
	i.resolveCurrentTx()

	if i.currentTx == nil {
		return nil
	}

	return i
}

// handleNextEVMAction increments evm iterator state based on nextEVMAction
func (i *EVMMempoolIterator) handleNextEVMAction() {
	switch i.nextEVMAction {
	case advance:
		if i.evmIterator != nil {
			i.evmIterator.Shift()
		}
	case skipAccount:
		if i.evmIterator != nil {
			i.evmIterator.Pop()
		}
	case none:
		// no action
	}
	i.nextEVMAction = none
}

// handleNextCosmosAction increments cosmos iterator state based on
// nextCosmosAction
func (i *EVMMempoolIterator) handleNextCosmosAction() {
	switch i.nextCosmosAction {
	case advance:
		if i.cosmosIterator != nil {
			i.cosmosIterator = i.cosmosIterator.Next()
		}
	case skipAccount:
		// no action for cosmos
	case none:
		// no action
	}
	i.nextCosmosAction = none
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
		i.nextEVMAction, i.nextCosmosAction = none, none
		i.currentTx = nil
		return
	}

	useEVM := i.compareFeePriority(evmTx, evmFee, cosmosTx, cosmosFee)

	if useEVM {
		sdkTx, err := i.convertEVMToSDKTx(evmTx)
		if err == nil {
			i.nextEVMAction = advance
			i.currentTx = sdkTx
			return
		}
		i.logger.Error("EVM transaction conversion failed, falling back to Cosmos transaction", "tx_hash", evmTx.Hash, "err", err)

		// conversion failed, this tx will not be included in the list of
		// returned txs by this iterator, therefore, fall future txs for this
		// account will be invalid, thus we should skip all future txs for this
		// account
		i.nextEVMAction = skipAccount

		// we are skipping the above account, and falling back to using the
		// current cosmos tx. technically, the next accounts evm tx may be a
		// higher fee
	}

	i.nextCosmosAction = advance
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

	// both have transactions - compare fees
	if cosmosFee.IsZero() {
		return true // use EVM if Cosmos transaction has no valid fee
	}

	// prefer EVM unless Cosmos has strictly higher fee
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
