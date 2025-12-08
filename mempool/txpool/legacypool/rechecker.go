package legacypool

import (
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/core/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

type (
	TxConverter interface {
		EVMTxToCosmosTx(tx *ethtypes.Transaction) (sdk.Tx, error)
	}

	RecheckCtxFn func(db vm.StateDB, header *types.Header) sdk.Context

	rechecker struct {
		ctx         sdk.Context
		txConverter TxConverter
		anteHandler sdk.AnteHandler
	}
)

func newRechecker(ctx sdk.Context, txConverter TxConverter) *rechecker {
	return &rechecker{
		ctx:         ctx,
		txConverter: txConverter,
	}
}

// Pending rechecks a tx in the pending pool. Note this is not thread safe with
// itself or the QueuedChecker.
func (r *rechecker) Pending(tx *ethtypes.Transaction) error {
	cosmosTx, err := r.txConverter.EVMTxToCosmosTx(tx)
	if err != nil {
		return fmt.Errorf("converting evm tx %s to cosmos tx: %w", tx.Hash(), err)
	}

	newCtx, err := r.anteHandler(r.ctx, cosmosTx, false)
	if err != nil {
		fmt.Printf("pending ante handler failed with err for tx %s (nonce %d) (write: %t): %s\n", tx.Hash(), tx.Nonce(), err.Error())
	} else {
		fmt.Printf("pending ante handler success for tx %s (nonce %d) (write: %t)\n", tx.Hash(), tx.Nonce())
	}
	if !newCtx.IsZero() {
		// Directly modify the ctx used for all future rechecks at this height.
		fmt.Printf("writing pending ante handler updates to state\n")
		r.ctx = newCtx
	}
	return err
}

// NewQueuedChecker returns a function that can be used to recheck queued
// transactions. Note that you should get a new QueuedChecker function every
// time you recheck a pending transaction via Pending. Note this is not thread
// safe with itself or Pending.
func (r *rechecker) NewQueuedChecker() func(tx *ethtypes.Transaction) error {
	// Create a cache ctx so that all queued txs that are checked using the
	// returned function act on the same context, but they will not persist the
	// changes outside of this queued checker. Note that Pending may update
	// r.ctx, so this may act on the context of previous Pending rechecks.
	ctx, _ := r.ctx.CacheContext()
	fmt.Println("creating queued rechecker with branched ctx")
	return func(tx *ethtypes.Transaction) error {
		cosmosTx, err := r.txConverter.EVMTxToCosmosTx(tx)
		if err != nil {
			return fmt.Errorf("converting evm tx %s to cosmos tx: %w", tx.Hash(), err)
		}

		newCtx, err := r.anteHandler(ctx, cosmosTx, false)
		if err != nil {
			fmt.Printf("queued ante handler failed with err for tx %s (nonce %d) (write: %t): %s\n", tx.Hash(), tx.Nonce(), err.Error())
		} else {
			fmt.Printf("queued ante handler success for tx %s (nonce %d) (write: %t)\n", tx.Hash(), tx.Nonce())
		}
		if !newCtx.IsZero() {
			// set the context back to the updated ante handler context and
			// set the ante handlers context to use the multistore that was
			// written to
			fmt.Printf("writing queued ante handler updates to branched ctx\n")
			ctx = newCtx
		}
		return tolerateAnteErr(err)
	}
}

// tolerateAnteErr returns nil if err is considered an error that should be
// ignored from the anteHandlers in the context of the recheckTxFn. If the
// error should not be ignored, it is returned unmodified.
func tolerateAnteErr(err error) error {
	if strings.Contains(err.Error(), "tx nonce is higher than account nonce") {
		return nil
	}
	return err
}
