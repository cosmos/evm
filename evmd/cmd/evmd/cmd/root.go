package cmd

import (
	"errors"
	types2 "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/evm/evmd/app/params"
	"github.com/cosmos/evm/evmd/cmd/evmd/cmd/testnet"
	"github.com/cosmos/evm/evmd/cmd/evmd/config"
	cosmosevmserverconfig "github.com/cosmos/evm/server/config"
	"io"
	"os"

	dbm "github.com/cosmos/cosmos-db"
	cosmosevmcmd "github.com/cosmos/evm/client"
	cosmosevmkeyring "github.com/cosmos/evm/crypto/keyring"
	eapp "github.com/cosmos/evm/evmd/app"
	cosmosevmserver "github.com/cosmos/evm/server"
	srvflags "github.com/cosmos/evm/server/flags"
	"github.com/cosmos/evm/x/vm/types"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	snapshottypes "cosmossdk.io/store/snapshots/types"
	storetypes "cosmossdk.io/store/types"
	confixcmd "cosmossdk.io/tools/confix/cmd"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	clientconfig "github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/debug"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/pruning"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/client/snapshot"
	"github.com/cosmos/cosmos-sdk/codec"
	sdkserver "github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	txmodule "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"

	cmtcfg "github.com/cometbft/cometbft/config"
	cmtcli "github.com/cometbft/cometbft/libs/cli"
)

// NewRootCmd creates a new root command.
func NewRootCmd() *cobra.Command {
	// we "pre"-instantiate the application for getting the injected/configured encoding configuration
	tempApp := eapp.New(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		simtestutil.EmptyAppOptions{},
	)
	encodingConfig := params.EncodingConfig{
		InterfaceRegistry: tempApp.InterfaceRegistry(),
		Codec:             tempApp.AppCodec(),
		TxConfig:          tempApp.GetTxConfig(),
		Amino:             tempApp.LegacyAmino(),
	}

	initClientCtx := client.Context{}.
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(authtypes.AccountRetriever{}).
		WithBroadcastMode(flags.FlagBroadcastMode).
		WithHomeDir(config.MustGetDefaultNodeHome()).
		WithViper("").
		WithLedgerHasProtobuf(true).
		// Cosmos EVM specific setup
		WithKeyringOptions(cosmosevmkeyring.Option())

	customAppTemplate, customAppConfig := config.InitAppConfig(types.DefaultEVMExtendedDenom, types.DefaultEVMChainID)

	rootCmd := &cobra.Command{
		Use:           eapp.AppName + "d",
		Short:         "EVMD cli",
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// set the default command outputs
			cmd.SetOut(cmd.OutOrStdout())
			cmd.SetErr(cmd.ErrOrStderr())

			initClientCtx = initClientCtx.WithCmdContext(cmd.Context())
			initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
			if err != nil {
				return err
			}

			initClientCtx, err = clientconfig.ReadFromClientConfig(initClientCtx)
			if err != nil {
				return err
			}

			// This needs to go after ReadFromClientConfig, as that function
			// sets the RPC client needed for SIGN_MODE_TEXTUAL. This sign mode
			// is only available if the client is online.
			if !initClientCtx.Offline {
				txConfigOpts := authtx.ConfigOptions{
					EnabledSignModes:           append(authtx.DefaultSignModes, signing.SignMode_SIGN_MODE_TEXTUAL),
					TextualCoinMetadataQueryFn: txmodule.NewGRPCCoinMetadataQueryFn(initClientCtx),
				}
				txConfigWithTextual, err := authtx.NewTxConfigWithOptions(
					codec.NewProtoCodec(encodingConfig.InterfaceRegistry),
					txConfigOpts,
				)
				if err != nil {
					return err
				}
				initClientCtx = initClientCtx.WithTxConfig(txConfigWithTextual)
			}

			if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
				return err
			}

			customTMConfig := initCometConfig()

			return sdkserver.InterceptConfigsPreRunHandler(cmd, customAppTemplate, customAppConfig, customTMConfig)
		},
	}

	initRootCmd(rootCmd, tempApp, customAppTemplate, customAppConfig.(cosmosevmserverconfig.Config))

	autoCliOpts := tempApp.AutoCliOpts()
	initClientCtx, _ = clientconfig.ReadFromClientConfig(initClientCtx)
	autoCliOpts.ClientCtx = initClientCtx

	if err := autoCliOpts.EnhanceRootCommand(rootCmd); err != nil {
		panic(err)
	}

	return rootCmd
}

// initCometConfig helps to override default CometBFT Config values.
// return cmtcfg.DefaultConfig if no custom configuration is required for the application.
func initCometConfig() *cmtcfg.Config {
	cfg := cmtcfg.DefaultConfig()

	// these values put a higher strain on node memory
	// cfg.P2P.MaxNumInboundPeers = 100
	// cfg.P2P.MaxNumOutboundPeers = 40

	return cfg
}

func initRootCmd(rootCmd *cobra.Command, app *eapp.App, appTemplate string, appConfig cosmosevmserverconfig.Config) {
	sdkAppCreator := func(l log.Logger, d dbm.DB, w io.Writer, ao servertypes.AppOptions) servertypes.Application {
		return newApp(l, d, w, ao)
	}
	defaultNodeHome := config.MustGetDefaultNodeHome()

	rootCmd.AddCommand(
		initInitCmd(app.BasicModuleManager, defaultNodeHome),
		genutilcli.Commands(app.GetTxConfig(), app.BasicModuleManager, defaultNodeHome),
		cmtcli.NewCompletionCmd(rootCmd, true),
		debug.Cmd(),
		confixcmd.ConfigCommand(),
		pruning.Cmd(sdkAppCreator, defaultNodeHome),
		snapshot.Cmd(sdkAppCreator),
		testnet.NewTestnetCmd(app.BasicModuleManager, types2.GenesisBalancesIterator{}, testnet.AppCreator{}),
	)

	// add Cosmos EVM' flavored TM commands to start server, etc.
	cosmosevmserver.AddCommands(
		rootCmd,
		cosmosevmserver.NewDefaultStartOptions(newApp, defaultNodeHome),
		appExport,
		func(c *cobra.Command) {
			c.Flags().String(flags.FlagChainID, "", "The network chain ID")
			c.Flag(srvflags.EVMChainID).DefValue = "0"
			if err := c.MarkFlagRequired(srvflags.EVMChainID); err != nil {
				panic(err) // should never happen
			}
		},
	)

	// add Cosmos EVM key commands
	rootCmd.AddCommand(
		cosmosevmcmd.KeyCommands(defaultNodeHome, true),
	)

	// add keybase, auxiliary RPC, query, genesis, and tx child commands
	rootCmd.AddCommand(
		sdkserver.StatusCommand(),
		txCommand(),
		queryCommand(),
	)
}

func initInitCmd(mbm module.BasicManager, defaultNodeHome string) *cobra.Command {
	c := genutilcli.InitCmd(mbm, defaultNodeHome)
	if err := c.MarkFlagRequired(flags.FlagChainID); err != nil {
		panic(err) // should never happen
	}
	return c
}

func queryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "query",
		Aliases:                    []string{"q"},
		Short:                      "Querying subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		rpc.ValidatorCommand(),
		sdkserver.QueryBlockCmd(),
		authcmd.QueryTxsByEventsCmd(),
		sdkserver.QueryBlocksCmd(),
		sdkserver.QueryBlockResultsCmd(),
		authcmd.QueryTxCmd(),
		authcmd.GetSimulateCmd(),
	)

	var err error
	_, err = srvflags.AddTxFlags(cmd)
	if err != nil {
		panic(err)
	}

	return cmd
}

func txCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "tx",
		Short:                      "Transactions subcommands",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		authcmd.GetSignCommand(),
		authcmd.GetSignBatchCommand(),
		authcmd.GetMultiSignCommand(),
		authcmd.GetMultiSignBatchCmd(),
		authcmd.GetValidateSignaturesCommand(),
		authcmd.GetBroadcastCommand(),
		authcmd.GetEncodeCommand(),
		authcmd.GetDecodeCommand(),
		authcmd.GetSimulateCmd(),
	)

	return cmd
}

// newApp creates the application
func newApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	appOpts servertypes.AppOptions,
) cosmosevmserver.Application {
	var cache storetypes.MultiStorePersistentCache

	if cast.ToBool(appOpts.Get(sdkserver.FlagInterBlockCache)) {
		cache = store.NewCommitKVStoreCacheManager()
	}

	pruningOpts, err := sdkserver.GetPruningOptionsFromFlags(appOpts)
	if err != nil {
		panic(err)
	}

	// get the cosmos chain id
	cosmosChainID, err := getCosmosChainIDFromOpts(appOpts)
	if err != nil {
		panic(err)
	}

	snapshotStore, err := sdkserver.GetSnapshotStore(appOpts)
	if err != nil {
		panic(err)
	}

	snapshotOptions := snapshottypes.NewSnapshotOptions(
		cast.ToUint64(appOpts.Get(sdkserver.FlagStateSyncSnapshotInterval)),
		cast.ToUint32(appOpts.Get(sdkserver.FlagStateSyncSnapshotKeepRecent)),
	)

	baseappOptions := []func(*baseapp.BaseApp){
		baseapp.SetPruning(pruningOpts),
		baseapp.SetMinGasPrices(cast.ToString(appOpts.Get(sdkserver.FlagMinGasPrices))),
		baseapp.SetHaltHeight(cast.ToUint64(appOpts.Get(sdkserver.FlagHaltHeight))),
		baseapp.SetHaltTime(cast.ToUint64(appOpts.Get(sdkserver.FlagHaltTime))),
		baseapp.SetMinRetainBlocks(cast.ToUint64(appOpts.Get(sdkserver.FlagMinRetainBlocks))),
		baseapp.SetInterBlockCache(cache),
		baseapp.SetTrace(cast.ToBool(appOpts.Get(sdkserver.FlagTrace))),
		baseapp.SetIndexEvents(cast.ToStringSlice(appOpts.Get(sdkserver.FlagIndexEvents))),
		baseapp.SetSnapshot(snapshotStore, snapshotOptions),
		baseapp.SetIAVLCacheSize(cast.ToInt(appOpts.Get(sdkserver.FlagIAVLCacheSize))),
		baseapp.SetIAVLDisableFastNode(cast.ToBool(appOpts.Get(sdkserver.FlagDisableIAVLFastNode))),
		baseapp.SetChainID(cosmosChainID),
	}

	return eapp.New(
		logger, db, traceStore, true,
		appOpts,
		baseappOptions...,
	)
}

// appExport creates a new simapp (optionally at a given height) and exports state.
func appExport(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	height int64,
	forZeroHeight bool,
	jailAllowedAddrs []string,
	appOpts servertypes.AppOptions,
	modulesToExport []string,
) (servertypes.ExportedApp, error) {
	var app *eapp.App

	// this check is necessary as we use the flag in x/upgrade.
	// we can exit more gracefully by checking the flag here.
	homePath, ok := appOpts.Get(flags.FlagHome).(string)
	if !ok || homePath == "" {
		return servertypes.ExportedApp{}, errors.New("application home not set")
	}

	if height != -1 {
		app = eapp.New(logger, db, traceStore, false, appOpts)

		if err := app.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	} else {
		app = eapp.New(logger, db, traceStore, true, appOpts)
	}

	return app.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}

// getCosmosChainIDFromOpts returns the chain Id from app Opts
// It first tries to get from the chainId flag, if not available
// it will load from home
func getCosmosChainIDFromOpts(appOpts servertypes.AppOptions) (string, error) {
	// Get the chain Id from appOpts
	chainID := cast.ToString(appOpts.Get(flags.FlagChainID))
	if chainID != "" {
		return chainID, nil
	}

	homeDir := cast.ToString(appOpts.Get(flags.FlagHome))
	return config.GetChainIDFromHome(homeDir)
}
