package evmd

import (
	"fmt"

	"github.com/spf13/viper"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"

	evmconfig "github.com/cosmos/evm/config"
	evmmempool "github.com/cosmos/evm/mempool"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// configureEVMMempool sets up the EVM mempool and related handlers using viper configuration.
func (app *EVMD) configureEVMMempool(appOpts servertypes.AppOptions, logger log.Logger) error {
	if evmtypes.GetChainConfig() == nil {
		logger.Debug("evm chain config is not set, skipping mempool configuration")
		return nil
	}

	v, ok := appOpts.(*viper.Viper)
	if !ok {
		return fmt.Errorf("appOpts is not a viper instance")
	}

	blockGasLimit := evmconfig.GetBlockGasLimit(v, logger)
	minTip := evmconfig.GetMinTip(v, logger)

	// Get mempool configuration from app.toml
	mempoolConfig, err := evmconfig.GetMempoolConfig(v, logger)
	if err != nil {
		return fmt.Errorf("failed to get mempool config: %w", err)
	}

	// Set required fields that aren't configurable via app.toml
	mempoolConfig.AnteHandler = app.GetAnteHandler()
	mempoolConfig.BlockGasLimit = blockGasLimit
	mempoolConfig.MinTip = minTip

	evmMempool := evmmempool.NewExperimentalEVMMempool(
		app.CreateQueryContext,
		logger,
		app.EVMKeeper,
		app.FeeMarketKeeper,
		app.txConfig,
		app.clientCtx,
		mempoolConfig,
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
	app.SetPrepareProposal(abciProposalHandler.PrepareProposalHandler())

	return nil
}
