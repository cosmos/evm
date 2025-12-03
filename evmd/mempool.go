package evmd

import (
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/evm/server"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"

	evmmempool "github.com/cosmos/evm/mempool"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// enables abci.InsertTx & abci.ReapTxs to be used exclusively by the mempool.
// @see evmmempool.ExperimentalEVMMempool.OperateExclusively
const mempoolOperateExclusively = true

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

	// todo should be replaced with evmmempool.NewInsertTxHandler() as soon as it's implemented
	app.SetInsertTxHandler(func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		res, err := app.CheckTx(&abci.RequestCheckTx{Tx: req.Tx})
		if err != nil {
			return nil, err
		}

		return &abci.ResponseInsertTx{Code: res.Code}, nil
	})

	txVerifier := NewNoCheckProposalTxVerifier(app.BaseApp)
	abciProposalHandler := baseapp.NewDefaultProposalHandler(evmMempool, txVerifier)
	abciProposalHandler.SetSignerExtractionAdapter(
		evmmempool.NewEthSignerExtractionAdapter(
			sdkmempool.NewDefaultSignerExtractionAdapter(),
		),
	)
	app.SetPrepareProposal(abciProposalHandler.PrepareProposalHandler())

	return nil
}

// createMempoolConfig creates a new EVMMempoolConfig with the default configuration
// and overrides it with values from appOpts if they exist and are non-zero.
func (app *EVMD) createMempoolConfig(appOpts servertypes.AppOptions, logger log.Logger) (*evmmempool.EVMMempoolConfig, error) {
	return &evmmempool.EVMMempoolConfig{
		AnteHandler:        app.GetAnteHandler(),
		LegacyPoolConfig:   server.GetLegacyPoolConfig(appOpts, logger),
		BlockGasLimit:      server.GetBlockGasLimit(appOpts, logger),
		MinTip:             server.GetMinTip(appOpts, logger),
		OperateExclusively: mempoolOperateExclusively,
	}, nil
}
