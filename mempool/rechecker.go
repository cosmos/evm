package mempool

import (
	"fmt"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/utils"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type TxConverter interface {
	EVMTxToCosmosTx(tx *ethtypes.Transaction) (sdk.Tx, error)
}

// Rechecker runs recheckFn on pending and queued txs in the pool, given an
// sdk context via UpdateCtx.
//
// NOTE: Nonce of the recheckers functions are thread safe.
type Rechecker struct {
	// ctx is the context that pending pool rechecks should be run
	// on. updated only by pending pool txs running the recheckFn.
	ctx         sdk.Context
	anteHandler sdk.AnteHandler
	txConverter TxConverter
}

// NewRechecker creates a new rechecker that can recheck transactions.
func NewRechecker(anteHandler sdk.AnteHandler, txConverter TxConverter) *Rechecker {
	return &Rechecker{
		anteHandler: anteHandler,
		txConverter: txConverter,
	}
}

// GetContext returns a branched context. The caller can use the returned
// function in order to write updates applied to the returned context, back to
// the context stored by the rechecker for future callers to use.
func (r *Rechecker) GetContext() (sdk.Context, func()) {
	return r.ctx.CacheContext()
}

// Recheck revalidates a transaction against a context. It returns an updated
// context and an error that occurred while processing.
func (r *Rechecker) Recheck(ctx sdk.Context, tx *ethtypes.Transaction) (sdk.Context, error) {
	cosmosTx, err := r.txConverter.EVMTxToCosmosTx(tx)
	if err != nil {
		return sdk.Context{}, fmt.Errorf("converting evm tx %s to cosmos tx: %w", tx.Hash(), err)
	}

	return r.anteHandler(ctx, cosmosTx, false)
}

// Update updates the base context for rechecks based on the latest chain
// state.
func (r *Rechecker) Update(chain legacypool.BlockChain, header *ethtypes.Header) {
	bc, ok := chain.(*Blockchain)
	if !ok {
		panic("expected type *Blockchain for implementation of legacypool.BlockChain")
	}

	ctx, err := bc.GetLatestContext()
	if err != nil {
		panic(fmt.Errorf("could not get latest context on blockchain: %w", err))
	}

	if ctx.ConsensusParams().Block == nil {
		// set the latest blocks gas limit as the max gas in cp. this is
		// necessary to validate each tx's gas wanted
		maxGas, err := utils.SafeInt64(header.GasLimit)
		if err != nil {
			panic(fmt.Errorf("converting evm block gas limit to int64: %w", err))
		}
		cp := cmtproto.ConsensusParams{Block: &cmtproto.BlockParams{MaxGas: maxGas}}
		ctx = ctx.WithConsensusParams(cp)
	}
	r.ctx = ctx
}
