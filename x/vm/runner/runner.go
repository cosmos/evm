// Package runner installs a baseapp TxRunner wrapped with the EVM module's
// post-execution log-index fix-up (evmtypes.PatchTxResponses).
package runner

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SetRunner installs inner as bApp's block tx runner, wrapped so
// PatchTxResponses runs once per block. Works for sequential and BlockSTM
// runners alike; the SDK's SetBlockSTMTxRunner name is the same setter for
// both.
func SetRunner(bApp *baseapp.BaseApp, inner sdk.TxRunner) {
	bApp.SetBlockSTMTxRunner(Wrap(inner))
}

// Wrap returns a TxRunner that delegates to inner and then applies
// PatchTxResponses to the block results.
func Wrap(inner sdk.TxRunner) sdk.TxRunner {
	return &patchingRunner{inner: inner}
}

type patchingRunner struct {
	inner sdk.TxRunner
}

func (r *patchingRunner) Run(
	ctx context.Context,
	ms storetypes.MultiStore,
	txs [][]byte,
	deliverTx sdk.DeliverTxFunc,
) ([]*abci.ExecTxResult, error) {
	results, err := r.inner.Run(ctx, ms, txs, deliverTx)
	if err != nil {
		return nil, err
	}
	return evmtypes.PatchTxResponses(results)
}
