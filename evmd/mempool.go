package evmd

import (
	"context"
	"fmt"

	"github.com/cosmos/evm/server"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"

	evmmempool "github.com/cosmos/evm/mempool"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// configureEVMMempool sets up the EVM mempool and related handlers using viper configuration.
func (app *EVMD) configureEVMMempool(appOpts servertypes.AppOptions, logger log.Logger) error {
	if evmtypes.GetChainConfig() == nil {
		logger.Debug("evm chain config is not set, skipping mempool configuration")
		return nil
	}

	cosmosPoolMaxTx := server.GetCosmosPoolMaxTx(appOpts, logger)
	if cosmosPoolMaxTx < 0 {
		logger.Debug("app-side mempool is disabled, skipping evm mempool configuration")
		return nil
	}

	mempoolConfig, err := app.createMempoolConfig(appOpts, logger)
	if err != nil {
		return fmt.Errorf("failed to get mempool config: %w", err)
	}

	evmMempool := evmmempool.NewExperimentalEVMMempool(
		app.CreateQueryContext,
		logger,
		app.EVMKeeper,
		app.FeeMarketKeeper,
		app.txConfig,
		app.clientCtx,
		mempoolConfig,
		cosmosPoolMaxTx,
	)
	app.EVMMempool = evmMempool
	app.SetMempool(evmMempool)
	checkTxHandler := evmmempool.NewCheckTxHandler(evmMempool)
	app.SetCheckTxHandler(checkTxHandler)

	abciProposalHandler := baseapp.NewDefaultProposalHandler(evmMempool, app)
	abciProposalHandler.SetSignerExtractionAdapter(
		evmmempool.NewEthSignerExtractionAdapter(
			sdkmempool.NewDefaultSignerExtractionAdapter(),
		),
	)
	loggingTxSelector := NewLoggingTxSelector(logger, baseapp.NewDefaultTxSelector())
	abciProposalHandler.SetTxSelector(loggingTxSelector)
	app.SetPrepareProposal(abciProposalHandler.PrepareProposalHandler())

	return nil
}

type LoggingTxSelector struct {
	baseapp.TxSelector
	logger log.Logger
}

func NewLoggingTxSelector(logger log.Logger, txSelector baseapp.TxSelector) *LoggingTxSelector {
	return &LoggingTxSelector{TxSelector: txSelector, logger: logger}
}

func (selector *LoggingTxSelector) SelectedTxs(ctx context.Context) [][]byte {
	txs := selector.TxSelector.SelectedTxs(ctx)
	selector.logger.Info("selected txs for proposal", "num_txs", len(txs))
	return txs
}

func (selector *LoggingTxSelector) Clear() {
	selector.TxSelector.Clear()
}

func (selector *LoggingTxSelector) SelectTxForProposal(ctx context.Context, maxTxBytes, maxBlockGas uint64, memTx sdk.Tx, txBz []byte) bool {
	return selector.TxSelector.SelectTxForProposal(ctx, maxTxBytes, maxBlockGas, memTx, txBz)
}

// createMempoolConfig creates a new EVMMempoolConfig with the default configuration
// and overrides it with values from appOpts if they exist and are non-zero.
func (app *EVMD) createMempoolConfig(appOpts servertypes.AppOptions, logger log.Logger) (*evmmempool.EVMMempoolConfig, error) {
	return &evmmempool.EVMMempoolConfig{
		AnteHandler:      app.GetAnteHandler(),
		LegacyPoolConfig: server.GetLegacyPoolConfig(appOpts, logger),
		BlockGasLimit:    server.GetBlockGasLimit(appOpts, logger),
		MinTip:           server.GetMinTip(appOpts, logger),
	}, nil
}
