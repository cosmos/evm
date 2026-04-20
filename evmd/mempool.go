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
)

const (
	CodeTypeNoRetry = 1
)

// configureEVMMempool sets up the EVM mempool and related handlers using viper configuration.
func (app *EVMD) configureEVMMempool(appOpts servertypes.AppOptions, logger log.Logger) error {
	if evmtypes.GetChainConfig() == nil {
		logger.Debug("evm chain config is not set, skipping mempool configuration")
		return nil
	}

	var (
		mpConfig = server.ResolveMempoolConfig(app.GetAnteHandler(), appOpts, logger)

		txEncoder       = evmmempool.NewTxEncoder(app.txConfig)
		evmRechecker    = evmmempool.NewTxRechecker(mpConfig.AnteHandler, txEncoder)
		cosmosRechecker = evmmempool.NewTxRechecker(mpConfig.AnteHandler, txEncoder)
		cosmosPoolMaxTx = server.GetCosmosPoolMaxTx(appOpts, logger)
	)

	if cosmosPoolMaxTx < 0 {
		logger.Debug("evm mempool is disabled, skipping configuration")
		return nil
	}

	// create mempool
	mempool := evmmempool.NewMempool(
		app.CreateQueryContext,
		logger,
		app.EVMKeeper,
		app.FeeMarketKeeper,
		app.txConfig,
		evmRechecker,
		cosmosRechecker,
		mpConfig,
		cosmosPoolMaxTx,
	)

	app.EVMMempool = mempool

	// create ABCI handlers
	insertTxHandler := app.NewInsertTxHandler(mempool)
	reapTxsHandler := app.NewReapTxsHandler(mempool)
	prepareProposalHandler := baseapp.
		NewDefaultProposalHandler(mempool, NewNoCheckProposalTxVerifier(app.BaseApp)).
		PrepareProposalHandler()

	// set handlers and the mempool
	app.SetPrepareProposal(prepareProposalHandler)
	app.SetInsertTxHandler(insertTxHandler)
	app.SetReapTxsHandler(reapTxsHandler)
	app.SetMempool(mempool)

	return nil
}

func (app *EVMD) NewInsertTxHandler(evmMempool *evmmempool.Mempool) sdk.InsertTxHandler {
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

func (app *EVMD) NewReapTxsHandler(evmMempool *evmmempool.Mempool) sdk.ReapTxsHandler {
	return func(req *abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
		maxBytes, maxGas := req.GetMaxBytes(), req.GetMaxGas()
		txs, err := evmMempool.ReapNewValidTxs(maxBytes, maxGas)
		if err != nil {
			return nil, fmt.Errorf("reaping new valid txs from evm mempool with %d max bytes and %d max gas: %w", maxBytes, maxGas, err)
		}
		return &abci.ResponseReapTxs{Txs: txs}, nil
	}
}
