package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	cmtcfg "github.com/cometbft/cometbft/config"
	cmtcli "github.com/cometbft/cometbft/libs/cli"

	dbm "github.com/cosmos/cosmos-db"
	cosmosevmcmd "github.com/cosmos/evm/client"
	cosmosevmkeyring "github.com/cosmos/evm/crypto/keyring"
	"github.com/cosmos/evm/evmd"
	evmdconfig "github.com/cosmos/evm/evmd/cmd/evmd/config"
	cosmosevmserver "github.com/cosmos/evm/server"
	srvflags "github.com/cosmos/evm/server/flags"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	"cosmossdk.io/store"
	snapshottypes "cosmossdk.io/store/snapshots/types"
	storetypes "cosmossdk.io/store/types"
	confixcmd "cosmossdk.io/tools/confix/cmd"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	clientcfg "github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/debug"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/pruning"
	"github.com/cosmos/cosmos-sdk/client/rpc"
	"github.com/cosmos/cosmos-sdk/client/snapshot"
	sdkserver "github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	txmodule "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	genutilcli "github.com/cosmos/cosmos-sdk/x/genutil/client/cli"
)

// NewRootCmd creates a new root command for evmd with a cleaner, more understandable flow.
// It eliminates hardcoded constants and the confusing tempApp no-op pattern.
func NewRootCmd() *cobra.Command {
	// Initialize the SDK configuration
	initSDKConfig()

	// Get the default node home directory
	defaultNodeHome := evmdconfig.MustGetDefaultNodeHome()

	// Create a basic app instance for encoding config extraction
	// We use minimal configuration since this is only for getting codecs/encoders
	tempApp := createBasicApp(defaultNodeHome)

	encodingConfig := sdktestutil.TestEncodingConfig{
		InterfaceRegistry: tempApp.InterfaceRegistry(),
		Codec:             tempApp.AppCodec(),
		TxConfig:          tempApp.GetTxConfig(),
		Amino:             tempApp.LegacyAmino(),
	}
	// Initialize client context with encoding config
	initClientCtx := client.Context{}.
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(encodingConfig.TxConfig).
		WithLegacyAmino(encodingConfig.Amino).
		WithInput(os.Stdin).
		WithAccountRetriever(authtypes.AccountRetriever{}).
		WithBroadcastMode(flags.FlagBroadcastMode).
		WithHomeDir(defaultNodeHome).
		WithViper(""). // In simapp, we don't use any prefix for env variables.
		// Cosmos EVM specific setup
		WithKeyringOptions(cosmosevmkeyring.Option()).
		WithLedgerHasProtobuf(true)

	// Create root command
	rootCmd := &cobra.Command{
		Use:   "evmd",
		Short: "exemplary Cosmos EVM app",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return setupCommand(cmd, initClientCtx)
		},
	}

	// Add all subcommands
	addSubcommands(rootCmd, tempApp, defaultNodeHome)

	// Setup AutoCLI
	setupAutoCLI(rootCmd, tempApp, initClientCtx)

	return rootCmd
}

// initSDKConfig initializes the SDK configuration
func initSDKConfig() {
	cfg := sdk.GetConfig()
	evmdconfig.SetBip44CoinType(cfg)
	cfg.Seal()
}

// createBasicApp creates a basic app instance for extracting encoding configuration
func createBasicApp(nodeHome string) *evmd.EVMD {
	// No-op EVM options for codec extraction only
	noOpEvmAppOptions := func(_ uint64) error {
		return nil
	}

	return evmd.NewExampleApp(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		simtestutil.EmptyAppOptions{},
		1, // Temporary chain ID, only for codec extraction
		noOpEvmAppOptions,
	)
}

// setupCommand handles the command setup that runs before each command execution.
func setupCommand(cmd *cobra.Command, initClientCtx client.Context) error {
	// Set command outputs
	cmd.SetOut(cmd.OutOrStdout())
	cmd.SetErr(cmd.ErrOrStderr())

	// Update client context from command
	initClientCtx = initClientCtx.WithCmdContext(cmd.Context())

	// Read persistent command flags
	clientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
	if err != nil {
		return err
	}

	// Read from client config file
	clientCtx, err = clientcfg.ReadFromClientConfig(clientCtx)
	if err != nil {
		return err
	}

	// Setup enhanced transaction config if online
	if !clientCtx.Offline {
		clientCtx, err = setupEnhancedTxConfig(clientCtx)
		if err != nil {
			return err
		}
	}

	// Set client context handler
	if err := client.SetCmdClientContextHandler(clientCtx, cmd); err != nil {
		return err
	}

	// Check if this is an init command that should use minimal configuration
	if isInitCommand(cmd) {
		// For init commands, use minimal configuration with defaults
		chainConfig := evmdconfig.ChainConfig{
			ChainInfo: evmdconfig.ChainInfo{
				ChainID:    "",   // Will be set by the init command
				EVMChainID: 9001, // Default EVM chain ID
			},
			CoinInfo: evmtypes.EvmCoinInfo{
				Denom:         "atest",
				ExtendedDenom: "atest",
				DisplayDenom:  "test",
				Decimals:      evmtypes.EighteenDecimals,
			},
		}

		// Configure EVM with minimal configuration to prevent nil pointer dereferences
		// This is essential for commands like 'gentx' that trigger genesis validation
		if err := evmdconfig.EvmAppOptions(chainConfig); err != nil {
			return fmt.Errorf("failed to configure EVM for init command: %w", err)
		}

		customAppTemplate, customAppConfig := evmdconfig.InitAppConfig(chainConfig)
		customTMConfig := initCometConfig()
		return sdkserver.InterceptConfigsPreRunHandler(cmd, customAppTemplate, customAppConfig, customTMConfig)
	}

	// Load chain configuration from app options for non-init commands
	chainConfig, err := loadChainConfigFromContext(cmd, clientCtx)
	if err != nil {
		return err
	}

	// Initialize app configuration with loaded chain config
	customAppTemplate, customAppConfig := evmdconfig.InitAppConfig(chainConfig)
	customTMConfig := initCometConfig()

	// Apply EVM app options if we have a chain ID
	if chainConfig.ChainInfo.ChainID != "" {
		if err := evmdconfig.EvmAppOptions(chainConfig); err != nil {
			return err
		}
	}

	// Intercept and handle configurations
	return sdkserver.InterceptConfigsPreRunHandler(cmd, customAppTemplate, customAppConfig, customTMConfig)
}

// isInitCommand checks if the current command is an init command that should use minimal configuration
func isInitCommand(cmd *cobra.Command) bool {
	// Check the command name and path to identify init-like commands
	cmdPath := cmd.CommandPath()
	cmdName := cmd.Name()

	// Commands that need minimal configuration (usually run before full config exists)
	return strings.Contains(cmdPath, " init") ||
		strings.Contains(cmdPath, " collect-gentxs") ||
		strings.Contains(cmdPath, " gentx") ||
		strings.Contains(cmdPath, " add-genesis-account") ||
		strings.Contains(cmdPath, " validate-genesis") ||
		strings.Contains(cmdPath, " keys ") ||
		strings.Contains(cmdPath, " genesis ") ||
		strings.Contains(cmdPath, " config ") ||
		cmdName == "init" ||
		cmdName == "gentx" ||
		cmdName == "collect-gentxs" ||
		cmdName == "add-genesis-account" ||
		cmdName == "validate-genesis" ||
		cmdName == "add" || // for keys add
		cmdName == "set" || // for config set
		(strings.Contains(cmdPath, "keys") && cmdName == "add") ||
		(strings.Contains(cmdPath, "config") && cmdName == "set")
}

// setupEnhancedTxConfig sets up enhanced transaction configuration for online mode.
func setupEnhancedTxConfig(clientCtx client.Context) (client.Context, error) {
	enabledSignModes := append(tx.DefaultSignModes, signing.SignMode_SIGN_MODE_TEXTUAL) //nolint:gocritic
	txConfigOpts := tx.ConfigOptions{
		EnabledSignModes:           enabledSignModes,
		TextualCoinMetadataQueryFn: txmodule.NewGRPCCoinMetadataQueryFn(clientCtx),
	}
	txConfig, err := tx.NewTxConfigWithOptions(
		clientCtx.Codec,
		txConfigOpts,
	)
	if err != nil {
		return clientCtx, err
	}
	return clientCtx.WithTxConfig(txConfig), nil
}

// loadChainConfigFromContext loads chain configuration from the command context and flags.
func loadChainConfigFromContext(cmd *cobra.Command, clientCtx client.Context) (evmdconfig.ChainConfig, error) {
	// Create a mock app options from the command flags and viper
	viper := viper.GetViper()

	// Bind relevant flags to viper
	if err := bindConfigFlags(cmd, viper); err != nil {
		return evmdconfig.ChainConfig{}, err
	}

	// Load configuration using our config loader
	return evmdconfig.LoadChainConfig(viper)
}

// bindConfigFlags binds relevant configuration flags to viper.
func bindConfigFlags(cmd *cobra.Command, v *viper.Viper) error {
	// Bind chain ID flag
	if flag := cmd.Flags().Lookup(flags.FlagChainID); flag != nil {
		if err := v.BindPFlag(flags.FlagChainID, flag); err != nil {
			return err
		}
	}

	// Bind home flag
	if flag := cmd.Flags().Lookup(flags.FlagHome); flag != nil {
		if err := v.BindPFlag(flags.FlagHome, flag); err != nil {
			return err
		}
	}

	// Bind EVM chain ID flag
	if flag := cmd.Flags().Lookup(srvflags.EVMChainID); flag != nil {
		if err := v.BindPFlag(srvflags.EVMChainID, flag); err != nil {
			return err
		}
	}

	return nil
}

// addSubcommands adds all subcommands to the root command.
func addSubcommands(rootCmd *cobra.Command, app *evmd.EVMD, defaultNodeHome string) {
	// Create app creator function
	sdkAppCreator := func(l log.Logger, d dbm.DB, w io.Writer, ao servertypes.AppOptions) servertypes.Application {
		return newApp(l, d, w, ao)
	}

	// Add genesis and utility commands
	rootCmd.AddCommand(
		genutilcli.InitCmd(app.BasicModuleManager, defaultNodeHome),
		genutilcli.Commands(app.TxConfig(), app.BasicModuleManager, defaultNodeHome),
		cmtcli.NewCompletionCmd(rootCmd, true),
		debug.Cmd(),
		confixcmd.ConfigCommand(),
		pruning.Cmd(sdkAppCreator, defaultNodeHome),
		snapshot.Cmd(sdkAppCreator),
		NewTestnetCmd(app.BasicModuleManager, banktypes.GenesisBalancesIterator{}, appCreator{}),
	)

	// Add Cosmos EVM server commands
	cosmosevmserver.AddCommands(
		rootCmd,
		cosmosevmserver.NewDefaultStartOptions(newApp, defaultNodeHome),
		appExport,
		addModuleInitFlags,
	)

	// Add key management commands
	rootCmd.AddCommand(
		cosmosevmcmd.KeyCommands(defaultNodeHome, true),
	)

	// Add query, tx, and status commands
	rootCmd.AddCommand(
		sdkserver.StatusCommand(),
		queryCommand(),
		txCommand(),
	)

	// Add transaction flags
	if _, err := srvflags.AddTxFlags(rootCmd); err != nil {
		panic(err)
	}
}

// setupAutoCLI sets up the AutoCLI functionality.
func setupAutoCLI(rootCmd *cobra.Command, app *evmd.EVMD, clientCtx client.Context) {
	autoCliOpts := app.AutoCliOpts()
	clientCtx, _ = clientcfg.ReadFromClientConfig(clientCtx)
	autoCliOpts.ClientCtx = clientCtx

	if err := autoCliOpts.EnhanceRootCommand(rootCmd); err != nil {
		panic(err)
	}
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

func addModuleInitFlags(_ *cobra.Command) {}

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
		rpc.QueryEventForTxCmd(),
		rpc.ValidatorCommand(),
		authcmd.QueryTxsByEventsCmd(),
		authcmd.QueryTxCmd(),
		sdkserver.QueryBlockCmd(),
		sdkserver.QueryBlockResultsCmd(),
	)

	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

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

	cmd.PersistentFlags().String(flags.FlagChainID, "", "The network chain ID")

	return cmd
}

// newApp creates the application instance with configuration loaded from app options.
func newApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	appOpts servertypes.AppOptions,
) cosmosevmserver.Application {
	// Load chain configuration from app options
	chainConfig, err := evmdconfig.LoadChainConfig(appOpts)
	if err != nil {
		panic(fmt.Errorf("failed to load chain configuration: %w", err))
	}

	// Setup cache
	var cache storetypes.MultiStorePersistentCache
	if cast.ToBool(appOpts.Get(sdkserver.FlagInterBlockCache)) {
		cache = store.NewCommitKVStoreCacheManager()
	}

	// Get pruning options
	pruningOpts, err := sdkserver.GetPruningOptionsFromFlags(appOpts)
	if err != nil {
		panic(err)
	}

	// Use the loaded chain ID from configuration
	chainID := chainConfig.ChainInfo.ChainID
	if chainID == "" {
		// Fallback to trying to get from opts if not in config
		chainID, err = getChainIDFromOpts(appOpts)
		if err != nil {
			panic(err)
		}
	}

	// Get snapshot configuration
	snapshotStore, err := sdkserver.GetSnapshotStore(appOpts)
	if err != nil {
		panic(err)
	}

	snapshotOptions := snapshottypes.NewSnapshotOptions(
		cast.ToUint64(appOpts.Get(sdkserver.FlagStateSyncSnapshotInterval)),
		cast.ToUint32(appOpts.Get(sdkserver.FlagStateSyncSnapshotKeepRecent)),
	)

	// Configure BaseApp options
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
		baseapp.SetChainID(chainID),
	}

	// Create the application with loaded configuration
	return evmd.NewExampleApp(
		logger, db, traceStore, true,
		appOpts,
		chainConfig.ChainInfo.EVMChainID,
		evmdconfig.EvmAppOptionsFromConfig(chainConfig),
		baseappOptions...,
	)
}

// appExport creates a new application (optionally at a given height) and exports state.
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
	var exampleApp *evmd.EVMD

	// this check is necessary as we use the flag in x/upgrade.
	// we can exit more gracefully by checking the flag here.
	homePath, ok := appOpts.Get(flags.FlagHome).(string)
	if !ok || homePath == "" {
		return servertypes.ExportedApp{}, errors.New("application home not set")
	}

	viperAppOpts, ok := appOpts.(*viper.Viper)
	if !ok {
		return servertypes.ExportedApp{}, errors.New("appOpts is not viper.Viper")
	}

	// overwrite the FlagInvCheckPeriod
	viperAppOpts.Set(sdkserver.FlagInvCheckPeriod, 1)
	appOpts = viperAppOpts

	// Load chain configuration
	chainConfig, err := evmdconfig.LoadChainConfig(appOpts)
	if err != nil {
		return servertypes.ExportedApp{}, fmt.Errorf("failed to load chain configuration for export: %w", err)
	}

	// Use loaded chain ID
	chainID := chainConfig.ChainInfo.ChainID
	if chainID == "" {
		chainID, err = getChainIDFromOpts(appOpts)
		if err != nil {
			return servertypes.ExportedApp{}, err
		}
	}

	// Create app at specific height or latest
	if height != -1 {
		exampleApp = evmd.NewExampleApp(
			logger, db, traceStore, false, appOpts,
			chainConfig.ChainInfo.EVMChainID,
			evmdconfig.EvmAppOptionsFromConfig(chainConfig),
			baseapp.SetChainID(chainID),
		)

		if err := exampleApp.LoadHeight(height); err != nil {
			return servertypes.ExportedApp{}, err
		}
	} else {
		exampleApp = evmd.NewExampleApp(
			logger, db, traceStore, true, appOpts,
			chainConfig.ChainInfo.EVMChainID,
			evmdconfig.EvmAppOptionsFromConfig(chainConfig),
			baseapp.SetChainID(chainID),
		)
	}

	return exampleApp.ExportAppStateAndValidators(forZeroHeight, jailAllowedAddrs, modulesToExport)
}

// getChainIDFromOpts returns the chain Id from app Opts
// It first tries to get from the chainId flag, if not available
// it will load from home
func getChainIDFromOpts(appOpts servertypes.AppOptions) (chainID string, err error) {
	// Get the chain Id from appOpts
	chainID = cast.ToString(appOpts.Get(flags.FlagChainID))
	if chainID == "" {
		// If not available load from home
		homeDir := cast.ToString(appOpts.Get(flags.FlagHome))
		chainID, err = evmdconfig.GetChainIDFromHome(homeDir)
		if err != nil {
			return "", err
		}
	}

	return
}
