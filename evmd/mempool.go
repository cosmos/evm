package evmd

import (
	"errors"
	"fmt"

	"github.com/cosmos/evm/server"

	"cosmossdk.io/log"

	abci "github.com/cometbft/cometbft/abci/types"
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
	app.SetInsertTxHandler(app.NewInsertTxHandler(evmMempool))
	app.SetReapTxsHandler(app.NewReapTxsHandler(evmMempool))

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
		AnteHandler:      app.GetAnteHandler(),
		LegacyPoolConfig: server.GetLegacyPoolConfig(appOpts, logger),
		BlockGasLimit:    server.GetBlockGasLimit(appOpts, logger),
		MinTip:           server.GetMinTip(appOpts, logger),
	}, nil
}

const (
	CodeTypeNoRetry = abci.CodeTypeRetry + 1
)

func (app *EVMD) NewInsertTxHandler(evmMempool *evmmempool.ExperimentalEVMMempool) sdk.InsertTxHandler {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		txBytes := req.GetTx()

		tx, err := app.TxDecode(txBytes)
		if err != nil {
			// TODO: is this the right response here for completely invalid txs
			// that we cannot process?
			return &abci.ResponseInsertTx{Code: CodeTypeNoRetry}, fmt.Errorf("decoding tx: %w", err)
		}

		code := abci.CodeTypeOK
		if err := evmMempool.InsertEVMTxAynsc(tx); err != nil {
			if errors.Is(err, evmmempool.ErrMempoolFull) {
				code = abci.CodeTypeRetry
			} else {
				code = CodeTypeNoRetry
			}
		}

		return &abci.ResponseInsertTx{Code: code}, nil
	}
}

func (app *EVMD) NewReapTxsHandler(evmMempool *evmmempool.ExperimentalEVMMempool) sdk.ReapTxsHandler {
	return func(req *abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
		return nil, nil
	}
}
