package evmd

import (
	"errors"
	"fmt"

	evmmempool "github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/server"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log/v2"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
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

	txEncoder := evmmempool.NewTxEncoder(app.txConfig)

	if server.GetShouldOperateExclusively(appOpts, logger) {
		logger.Info("app-side mempool is operating exclusively, setting up Krakatoa mempool")

		krakatoaConfig := app.createKrakatoaMempoolConfig(appOpts, logger)
		evmRechecker := evmmempool.NewTxRechecker(krakatoaConfig.AnteHandler, txEncoder)
		cosmosRechecker := evmmempool.NewTxRechecker(krakatoaConfig.AnteHandler, txEncoder)

		krakatoaMempool := evmmempool.NewKrakatoaMempool(
			app.CreateQueryContext,
			logger,
			app.EVMKeeper,
			app.FeeMarketKeeper,
			app.txConfig,
			txEncoder,
			evmRechecker,
			cosmosRechecker,
			krakatoaConfig,
			cosmosPoolMaxTx,
		)

		app.SetInsertTxHandler(app.NewInsertTxHandler(krakatoaMempool))
		app.SetReapTxsHandler(app.NewReapTxsHandler(krakatoaMempool))

		txVerifier := NewNoCheckProposalTxVerifier(app.BaseApp)
		abciProposalHandler := baseapp.NewDefaultProposalHandler(krakatoaMempool, txVerifier)
		abciProposalHandler.SetSignerExtractionAdapter(
			evmmempool.NewEthSignerExtractionAdapter(
				sdkmempool.NewDefaultSignerExtractionAdapter(),
			),
		)
		app.SetPrepareProposal(abciProposalHandler.PrepareProposalHandler())

		app.EVMMempool = krakatoaMempool
		app.SetMempool(krakatoaMempool)
	} else {
		logger.Info("app-side mempool is not operating exclusively, setting up default EVM mempool")

		evmMempool := evmmempool.NewExperimentalEVMMempool(
			app.CreateQueryContext,
			logger,
			app.EVMKeeper,
			app.FeeMarketKeeper,
			app.txConfig,
			app.createMempoolConfig(appOpts, logger),
			cosmosPoolMaxTx,
		)

		app.SetCheckTxHandler(evmmempool.NewCheckTxHandler(evmMempool))

		app.EVMMempool = evmMempool
		app.SetMempool(evmMempool)
	}

	return nil
}

// createMempoolConfig creates a new EVMMempoolConfig with the default configuration
// and overrides it with values from appOpts if they exist and are non-zero.
func (app *EVMD) createMempoolConfig(appOpts servertypes.AppOptions, logger log.Logger) *evmmempool.EVMMempoolConfig {
	return &evmmempool.EVMMempoolConfig{
		AnteHandler:      app.GetAnteHandler(),
		LegacyPoolConfig: server.GetLegacyPoolConfig(appOpts, logger),
		BlockGasLimit:    server.GetBlockGasLimit(appOpts, logger),
		MinTip:           server.GetMinTip(appOpts, logger),
	}
}

// createKrakatoaMempoolConfig creates a new KrakatoaMempoolConfig with the default configuration
// and overrides it with values from appOpts if they exist and are non-zero.
func (app *EVMD) createKrakatoaMempoolConfig(appOpts servertypes.AppOptions, logger log.Logger) *evmmempool.KrakatoaMempoolConfig {
	mempoolConfig := app.createMempoolConfig(appOpts, logger)
	return &evmmempool.KrakatoaMempoolConfig{
		EVMMempoolConfig:         *mempoolConfig,
		PendingTxProposalTimeout: server.GetPendingTxProposalTimeout(appOpts, logger),
		InsertQueueSize:          server.GetMempoolInsertQueueSize(appOpts, logger),
	}
}

const (
	CodeTypeNoRetry = 1
)

func (app *EVMD) NewInsertTxHandler(evmMempool *evmmempool.KrakatoaMempool) sdk.InsertTxHandler {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		txBytes := req.GetTx()

		tx, err := app.TxDecode(txBytes)
		if err != nil {
			return nil, fmt.Errorf("decoding tx: %w", err)
		}

		ctx := app.GetContextForCheckTx(txBytes)

		code := abci.CodeTypeOK
		if err := evmMempool.InsertAsync(ctx, tx); err != nil {
			// since we are using InsertAsync here, the only errors that will
			// be returned are via the InsertQueue if it is full (for EVM txs),
			// in which case we should retry, or some level of validation
			// failed on a cosmos tx (CheckTx), invalid encoding, etc, in which
			// case we should not retry
			switch {
			case errors.Is(err, evmmempool.ErrQueueFull):
				code = abci.CodeTypeRetry
			default:
				code = CodeTypeNoRetry
			}
		}
		return &abci.ResponseInsertTx{Code: code}, nil
	}
}

func (app *EVMD) NewReapTxsHandler(evmMempool *evmmempool.KrakatoaMempool) sdk.ReapTxsHandler {
	return func(req *abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
		maxBytes, maxGas := req.GetMaxBytes(), req.GetMaxGas()
		txs, err := evmMempool.ReapNewValidTxs(maxBytes, maxGas)
		if err != nil {
			return nil, fmt.Errorf("reaping new valid txs from evm mempool with %d max bytes and %d max gas: %w", maxBytes, maxGas, err)
		}
		return &abci.ResponseReapTxs{Txs: txs}, nil
	}
}
