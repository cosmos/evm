package mempool

import (
	"fmt"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/cosmos/evm/utils"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type TxConverter interface {
	EVMTxToCosmosTx(tx *ethtypes.Transaction) (sdk.Tx, error)
	// CosmosTx returns the canonical cosmos-tx wire bytes. Used by the
	// rechecker to enforce tx.Size() ≤ ConsensusParams.Block.MaxBytes when
	// gov reduces MaxBytes below an admitted tx.
	CosmosTx(tx sdk.Tx) ([]byte, error)
}

// TxRechecker runs recheckFn on pending and queued txs in the pool, given an
// sdk context via UpdateCtx.
//
// NOTE: None of the recheckers functions are thread safe.
type TxRechecker struct {
	// ctx is the context that rechecks should be run on. Updated by calling
	// the returned function from GetContext.
	ctx sdk.Context

	anteHandler sdk.AnteHandler
	txConverter TxConverter
}

// NewTxRechecker creates a new rechecker that can recheck transactions.
func NewTxRechecker(anteHandler sdk.AnteHandler, txConverter TxConverter) *TxRechecker {
	return &TxRechecker{
		anteHandler: anteHandler,
		txConverter: txConverter,
	}
}

// GetContext returns a branched context. The caller can use the returned
// function in order to write updates applied to the returned context, back to
// the context stored by the rechecker for future callers to use.
//
// NOTE: This function is not thread safe with itself or any other Rechecker functions.
func (r *TxRechecker) GetContext() (sdk.Context, func()) {
	if r.ctx.MultiStore() == nil {
		return sdk.Context{}, func() {}
	}

	// CacheContext behavior, but dont emit events back to parent manager,
	// rechecking doesnt care about event and we will race on this if we do
	cms := r.ctx.MultiStore().CacheMultiStore()
	cc := r.ctx.WithMultiStore(cms).WithEventManager(sdk.NewEventManager())
	write := func() {
		cms.Write()
	}
	return cc, write
}

// RecheckEVM revalidates an EVM transaction against a context. It returns an updated
// context and an error that occurred while processing.
//
// NOTE: This function is not thread safe with itself or any other Rechecker functions.
func (r *TxRechecker) RecheckEVM(ctx sdk.Context, tx *ethtypes.Transaction) (sdk.Context, error) {
	cosmosTx, err := r.txConverter.EVMTxToCosmosTx(tx)
	if err != nil {
		return sdk.Context{}, fmt.Errorf("converting evm tx %s to cosmos tx: %w", tx.Hash(), err)
	}
	if err := r.checkMaxBytes(ctx, cosmosTx); err != nil {
		return ctx, err
	}
	return r.anteHandler(ctx, cosmosTx, false)
}

// RecheckCosmos revalidates a Cosmos transaction against a context. It returns an updated
// context and an error that occurred while processing.
//
// NOTE: This function is not thread safe with itself or any other Rechecker functions.
func (r *TxRechecker) RecheckCosmos(ctx sdk.Context, tx sdk.Tx) (sdk.Context, error) {
	if err := r.checkMaxBytes(ctx, tx); err != nil {
		return ctx, err
	}
	return r.anteHandler(ctx, tx, false)
}

// checkMaxBytes rejects txs whose encoded size exceeds the chain's current
// consensus MaxBytes. Comet enforces this on initial admission, but txs
// already in the mempool are not re-evaluated when gov lowers MaxBytes.
// Without this check, a now-oversized tx wedges the head of the reap list
// and blocks every later tx until lifetime expiry or nonce overtaking
// removes it. Skipping when MaxBytes is unset preserves behavior on chains
// without consensus params populated yet.
func (r *TxRechecker) checkMaxBytes(ctx sdk.Context, tx sdk.Tx) error {
	cp := ctx.ConsensusParams()
	if cp.Block == nil || cp.Block.MaxBytes <= 0 {
		return nil
	}
	bz, err := r.txConverter.CosmosTx(tx)
	if err != nil {
		return fmt.Errorf("encoding tx for size check: %w", err)
	}
	if int64(len(bz)) > cp.Block.MaxBytes {
		return fmt.Errorf("tx size %d exceeds block max bytes %d", len(bz), cp.Block.MaxBytes)
	}
	return nil
}

// Update updates the base context for rechecks based on the latest chain
// state. The caller provides the context directly.
//
// NOTE: This function is not thread safe with itself or any other Rechecker functions.
func (r *TxRechecker) Update(ctx sdk.Context, header *ethtypes.Header) {
	cached, _ := ctx.CacheContext()
	cached = cached.WithBlockGasMeter(storetypes.NewGasMeter(header.GasLimit))
	cached = cached.WithGasMeter(storetypes.NewInfiniteGasMeter())
	if cached.ConsensusParams().Block == nil {
		// set the latest blocks gas limit as the max gas in cp. this is
		// necessary to validate each tx's gas wanted
		maxGas, err := utils.SafeInt64(header.GasLimit)
		if err != nil {
			panic(fmt.Errorf("converting evm block gas limit to int64: %w", err))
		}
		cp := cmtproto.ConsensusParams{Block: &cmtproto.BlockParams{MaxGas: maxGas}}
		cached = cached.WithConsensusParams(cp)
	}
	r.ctx = cached
}
