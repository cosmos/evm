package mempool

import (
	"errors"
	"fmt"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/utils"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type TxConverter interface {
	EVMTxToCosmosTx(tx *ethtypes.Transaction) (sdk.Tx, error)
}

type Rechecker struct {
	// height is the only height that it is valid to use this instance of the
	// rechecker on. i.e. if the on chain height increments, create a new
	// instance of the rechecker.
	height uint64

	ctx sdk.Context

	txConverter TxConverter

	anteHandler sdk.AnteHandler
}

func NewRechecker(chain legacypool.BlockChain, txConverter TxConverter) *Rechecker {
	bc, ok := chain.(*Blockchain)
	if !ok {
		panic("unexpected type for BlockChain, expected *mempool.Blockchain")
	}

	ctx := baseRecheckContext(bc)
	return &Rechecker{
		height:      chain.CurrentBlock().Number.Uint64(),
		ctx:         ctx,
		txConverter: txConverter,
	}
}

// Pending rechecks a tx in the pending pool. Note this is not thread safe with
// itself or the QueuedChecker.
func (r *Rechecker) Pending(tx *ethtypes.Transaction) error {
	cosmosTx, err := r.txConverter.EVMTxToCosmosTx(tx)
	if err != nil {
		return fmt.Errorf("converting evm tx %s to cosmos tx: %w", tx.Hash(), err)
	}

	// Directly modify the ctx used for all future rechecks at this height.
	newCtx, err := r.anteHandler(r.ctx, cosmosTx, false)
	if err != nil {
		fmt.Printf("ante handler failed with err for tx %s (nonce %d) (write: %t): %s\n", tx.Hash(), tx.Nonce(), err.Error())
	} else {
		fmt.Printf("ante handler success for tx %s (nonce %d) (write: %t)\n", tx.Hash(), tx.Nonce())
	}
	if !newCtx.IsZero() {
		// set the context back to the updated ante handler context and
		// set the ante handlers context to use the multistore that was
		// written to
		fmt.Printf("writing ante handler updates to state\n")
		r.ctx = newCtx
	}
	return err
}

// NewQueuedChecker returns a function that can be used to recheck queued
// transactions. Note that you should get a new QueuedChecker function every
// time you recheck a pending transaction via Pending. Note this is not thread
// safe with itself or Pending.
func (r *Rechecker) NewQueuedChecker() func(tx *ethtypes.Transaction) error {
	// Create a cache ctx so that all queued txs that are checked using the
	// returned function act on the same context, but they will not persist the
	// changes outside of this queued checker. Note that Pending may update
	// r.ctx, so this may act on the context of previous Pending rechecks.
	ctx, _ := r.ctx.CacheContext()
	return func(tx *ethtypes.Transaction) error {
		cosmosTx, err := r.txConverter.EVMTxToCosmosTx(tx)
		if err != nil {
			return fmt.Errorf("converting evm tx %s to cosmos tx: %w", tx.Hash(), err)
		}

		_, err = r.anteHandler(ctx, cosmosTx, false)
		return tolerateAnteErr(err)
	}
}

// baseRecheckContext gets the base context that all rechecks will operate off
// of. This context may be later modified by invocations of Pending or the
// QueuedChecker.
func baseRecheckContext(bc *Blockchain) sdk.Context {
	ctx, err := bc.GetLatestContext()
	if err != nil {
		// TODO: we probably dont want to panic here, but for POC im saying
		// this is ok, the only real other option here is to nuke the
		// entire mempool, or force another recheck but we cant be sure
		// that will also not fail here
		panic(fmt.Errorf("getting latest context from blockchain: %w", err))
	}

	ms := ctx.MultiStore()
	msCache := ms.CacheMultiStore()
	ctx = ctx.WithMultiStore(msCache)

	if ctx.ConsensusParams().Block == nil {
		// set the latest blocks gas limit as the max gas in cp. this is
		// necessary to validate each tx's gas wanted
		maxGas, err := utils.SafeInt64(bc.CurrentBlock().GasLimit)
		if err != nil {
			panic(fmt.Errorf("converting evm block gas limit to int64: %w", err))
		}
		cp := cmtproto.ConsensusParams{Block: &cmtproto.BlockParams{MaxGas: maxGas}}
		ctx = ctx.WithConsensusParams(cp)
	}

	return ctx
}

// tolerateAnteErr returns nil if err is considered an error that should be
// ignored from the anteHandlers in the context of the recheckTxFn. If the
// error should not be ignored, it is returned unmodified.
func tolerateAnteErr(err error) error {
	if errors.Is(err, ErrNonceGap) {
		return nil
	}
	return err
}
