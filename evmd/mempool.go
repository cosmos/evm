package evmd

import (
	"errors"
	"fmt"

	"github.com/cosmos/evm/mempool/txpool"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
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
		evmmempool.NewTxEncoder(app.txConfig),
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
		AnteHandler:        app.GetAnteHandler(),
		LegacyPoolConfig:   server.GetLegacyPoolConfig(appOpts, logger),
		BlockGasLimit:      server.GetBlockGasLimit(appOpts, logger),
		MinTip:             server.GetMinTip(appOpts, logger),
		OperateExclusively: mempoolOperateExclusively,
	}, nil
}

const (
	CodeTypeNoRetry = 1
)

func (app *EVMD) NewInsertTxHandler(evmMempool *evmmempool.ExperimentalEVMMempool) sdk.InsertTxHandler {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		txBytes := req.GetTx()

		tx, err := app.TxDecode(txBytes)
		if err != nil {
			return nil, fmt.Errorf("decoding tx: %w", err)
		}

		ctx := app.GetContextForCheckTx(txBytes)

		code := abci.CodeTypeOK
		if err := evmMempool.InsertAsync(ctx, tx); err != nil {
			switch {
			case errors.Is(err, txpool.ErrAlreadyKnown):
				code = CodeTypeNoRetry
			case errors.Is(err, legacypool.ErrTxPoolOverflow) || errors.Is(err, txpool.ErrUnderpriced) || errors.Is(err, legacypool.ErrFutureReplacePending):
				// ErrUnderpriced is grouped here since this is returned if the
				// mempool is full but the tx cheaper than the cheapest tx in the
				// pool so it cannot bump another tx out
				//
				// ErrFutureReplacePending is grouped here since this is returned
				// if the tx pool is full and this tx is priced higher than the
				// cheapest tx in the pool (i.e. it is beneficial to accept it and
				// remove the cheaper txs). However this tx is also nonce gapped
				// (future), and to add it we must drop a tx from the pending pool.
				// Now this is actually not beneficial to add this tx since it may
				// not become executable for a long time, but the pending tx is
				// currently executable, so we opt to not add this tx. This will
				// only happen if the pool is full, so we simply return that the
				// pool is full so the user can wait until the pool is not full and
				// retry this tx.
				code = abci.CodeTypeRetry
			case errors.Is(err, txpool.ErrReplaceUnderpriced):
				// Submitting this tx again will result in the same error unless
				// the current tx it is trying to replace is discarded for some
				// reason, this is unlikely so we simply return that this tx is
				// invalid in order to signal to the user that they should modify
				// it before resubmission.
				fallthrough
			default:
				// failed some level of validation
				code = CodeTypeNoRetry
			}
		}
		return &abci.ResponseInsertTx{Code: code}, nil
	}
}

func (app *EVMD) NewReapTxsHandler(evmMempool *evmmempool.ExperimentalEVMMempool) sdk.ReapTxsHandler {
	return func(req *abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
		maxBytes, maxGas := req.GetMaxBytes(), req.GetMaxGas()
		txs, err := evmMempool.ReapNewValidTxs(maxBytes, maxGas)
		if err != nil {
			return nil, fmt.Errorf("reaping new valid txs from evm mempool with %d max bytes and %d max gas: %w", maxBytes, maxGas, err)
		}
		return &abci.ResponseReapTxs{Txs: txs}, nil
	}
}
