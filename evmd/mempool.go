package evmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

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
	CodeTypeNoRetry = uint32(1)
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
		checkTxTimeout  = server.GetMempoolCheckTxTimeout(appOpts, logger)
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
	prepareProposalHandler := baseapp.
		NewDefaultProposalHandler(mempool, NewNoCheckProposalTxVerifier(app.BaseApp)).
		PrepareProposalHandler()

	insertTxHandler := app.NewInsertTxHandler(mempool)
	reapTxsHandler := app.NewReapTxsHandler(mempool)
	checkTxHandler := app.NewCheckTxHandler(mempool, checkTxTimeout)

	// set handlers and the mempool
	app.SetPrepareProposal(prepareProposalHandler)
	app.SetInsertTxHandler(insertTxHandler)
	app.SetReapTxsHandler(reapTxsHandler)
	app.SetCheckTxHandler(checkTxHandler)

	app.SetMempool(mempool)

	return nil
}

// NewInsertTxHandler is the handler for ABCI.InsertTx. Used by CometBFT to asynchronously
// insert a new tx into the mempool. Supersedes ABCI.CheckTx. Handles concurrent requests
func (app *EVMD) NewInsertTxHandler(evmMempool *evmmempool.Mempool) sdk.InsertTxHandler {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		tx, err := app.TxDecode(req.GetTx())
		if err != nil {
			return nil, fmt.Errorf("decoding tx: %w", err)
		}

		code := abci.CodeTypeOK

		if err := evmMempool.InsertAsync(tx); err != nil {
			// since we are using InsertAsync here, the only errors that will
			// be returned are via the InsertQueue if it is full (for EVM txs),
			// in which case we should retry, or some level of validation
			// failed on a cosmos tx (CheckTx), invalid encoding, etc, in which
			// case we should not retry
			if errors.Is(err, evmmempool.ErrQueueFull) {
				code = abci.CodeTypeRetry
			} else {
				code = CodeTypeNoRetry
			}
		}

		return &abci.ResponseInsertTx{Code: code}, nil
	}
}

// NewReapTxsHandler is the handler for ABCI.ReapTxs. It's used by CometBFT
// to reap valid txs from the mempool and share them with other peers. Handles concurrent requests.
func (app *EVMD) NewReapTxsHandler(evmMempool *evmmempool.Mempool) sdk.ReapTxsHandler {
	return func(req *abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
		maxBytes, maxGas := req.GetMaxBytes(), req.GetMaxGas()

		txs, err := evmMempool.ReapNewValidTxs(maxBytes, maxGas)
		if err != nil {
			return nil, fmt.Errorf(
				"reaping new valid txs from evm mempool with %d max bytes and %d max gas: %w",
				maxBytes, maxGas, err,
			)
		}

		return &abci.ResponseReapTxs{Txs: txs}, nil
	}
}

// NewCheckTxHandler is the handler for ABCI.CheckTx. Note: it's async and doesn't expect the caller
// to acquire ABCI lock. Used ONLY to support BroadcastTxSync (cosmos rpc). All EVM txs
// should be inserted via InsertTx handler or EVM RPC.
func (app *EVMD) NewCheckTxHandler(evmMempool *evmmempool.Mempool, timeout time.Duration) sdk.CheckTxHandler {
	return func(_ sdk.RunTx, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
		if req.Type != abci.CheckTxType_New {
			return nil, fmt.Errorf("unsupported abci.RequestCheckTx.Type: %s", req.Type)
		}

		tx, err := app.TxDecode(req.GetTx())
		if err != nil {
			return nil, fmt.Errorf("decoding tx: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		if err := evmMempool.Insert(ctx, tx); err != nil {
			return sdkerrors.ResponseCheckTxWithEvents(err, 0, 0, nil, false), nil
		}

		return &abci.ResponseCheckTx{Code: abci.CodeTypeOK}, nil
	}
}
