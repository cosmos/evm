// Package runner installs a baseapp TxRunner wrapped with the EVM module's
// post-execution log-index fix-up (evmtypes.PatchTxResponses).
package runner

import (
	"context"

	abci "github.com/cometbft/cometbft/abci/types"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/baseapp/blockexec"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SetRunner installs the EVM block tx runner, wrapped so PatchTxResponses runs
// once per block. It defaults to Block-STM with pre-estimation enabled; the
// WithDefault* values apply only when appOpts omits the setting (e.g. tests),
// otherwise the flag and app.toml value win.
func SetRunner(
	bApp *baseapp.BaseApp,
	appOpts servertypes.AppOptions,
	stores []storetypes.StoreKey,
	txDecoder sdk.TxDecoder,
	coinDenom func(storetypes.MultiStore) string,
) {
	blockexec.Apply(bApp, appOpts, stores, txDecoder, coinDenom,
		blockexec.WithDefaultExecutor(config.BlockExecutorBlockSTM),
		blockexec.WithDefaultPreEstimate(true),
		blockexec.WithRunnerWrap(Wrap),
	)
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

// Unwrap exposes the underlying runner so BaseApp's parallel-execution guards
// can see through the wrapper.
func (r *patchingRunner) Unwrap() sdk.TxRunner { return r.inner }
