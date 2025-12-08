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
	// RecheckCtxFn is a function that fetches an sdk context, given an EVM
	// stateDB and block header.
	RecheckCtxFn func(db vm.StateDB, header *types.Header) sdk.Context

	// RecheckFn is a function that rechecks a tx for validity given a sdk
	// context. This should return an updated context and an error if the tx
	// could not be properly rechecked.
	RecheckFn func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error)

	// rechecker runs recheckFn on pending and queued txs in the pool, given an
	// sdk context. The context should not be manually updated, if a new block
	// arrives that updates the chains context, a new instance of the rechecker
	// should be created.
	rechecker struct {
		// pendingCtx is the context that pending rechecks should be run on.
		pendingCtx sdk.Context

		// ctx is the context that the recheckFn should be run with
		ctx sdk.Context

		// recheckFn is the function that performs the recheck of a tx
		recheckFn RecheckFn
	}
)

// newRechecker creates a new rechecker that can recheck transactions only for
// this ctx.
func newRechecker(ctx sdk.Context, recheckFn RecheckFn) *rechecker {
	return &rechecker{
		pendingCtx: ctx,
		ctx:        ctx,
		recheckFn:  recheckFn,
	}
}

// Pending rechecks a tx in the pending pool. Note this is not thread safe with
// itself or the QueuedChecker.
func (r *rechecker) Pending(tx *ethtypes.Transaction) error {
	newCtx, err := r.recheckFn(r.pendingCtx, tx)
	if err != nil {
		fmt.Printf("pending ante handler failed with err for tx %s (nonce %d) (write: %t): %s\n", tx.Hash(), tx.Nonce(), err.Error())
	} else {
		fmt.Printf("pending ante handler success for tx %s (nonce %d) (write: %t)\n", tx.Hash(), tx.Nonce())
	}
	if !newCtx.IsZero() {
		// Directly modify the ctx used for all future rechecks at this height.
		fmt.Printf("writing pending ante handler updates to state\n")
		r.pendingCtx = newCtx
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
	fmt.Println("creating queued rechecker with branched ctx")
	queueCtx, _ := r.ctx.CacheContext()
	return func(tx *ethtypes.Transaction) error {
		txCtx, _ := queueCtx.CacheContext()
		newCtx, err := r.recheckFn(txCtx, tx)
		if err != nil {
			fmt.Printf("queued ante handler failed with err for tx %s (nonce %d): %t): %s\n", tx.Hash(), tx.Nonce(), err.Error())
		} else {
			fmt.Printf("queued ante handler success for tx %s (nonce %d): %t)\n", tx.Hash(), tx.Nonce())
		}
		if !newCtx.IsZero() {
			// set the context back to the updated ante handler context
			fmt.Printf("writing queued ante handler updates to branched ctx\n")
			queueCtx = newCtx
			if tolerateAnteErr(err) == nil {
				// successful recheckFn, update the main context since this tx
				// will be promoted from queued to pending and we need future
				// queued checkers at this height to operate knowing about this
				// txns context updates
				fmt.Printf("writing queued ante handler updates to MAAINNN ctx")
				r.ctx = newCtx
			}
		}
		return tolerateAnteErr(err)
	}
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
