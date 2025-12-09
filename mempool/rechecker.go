package mempool

import (
	"fmt"
	"strings"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/utils"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

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

func (r *Rechecker) GetContext() (sdk.Context, func()) {
	return r.ctx.CacheContext()
}

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

// tolerateAnteErr returns nil if err is considered an error that should be
// ignored from the recheckFn. If the error should not be ignored, it is
// returned unmodified.
func tolerateAnteErr(err error) error {
	// TODO: this is awful, we should not be checking the error string here,
	// but importing this error from the mempool package is an import cycle.
	if err == nil || strings.Contains(err.Error(), "tx nonce is higher than account nonce") {
		return nil
	}
	return err
}
