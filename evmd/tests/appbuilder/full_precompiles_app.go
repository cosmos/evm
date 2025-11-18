package appbuilder

import (
	"encoding/json"
	"fmt"
	"os"
	goruntime "runtime"

	evm "github.com/cosmos/evm"
	evmconfig "github.com/cosmos/evm/evmd/cmd/evmd/config"
	"github.com/cosmos/evm/evmd/tests/integration"
	evmencoding "github.com/cosmos/evm/encoding"
	precompilestypes "github.com/cosmos/evm/precompiles/types"
	"github.com/cosmos/evm/x/erc20"
	erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/feemarket"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/cosmos/evm/x/vm"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cast"

	_ "github.com/ethereum/go-ethereum/eth/tracers/js"
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"

	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	reflectionv1 "cosmossdk.io/api/cosmos/reflection/v1"
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/upgrade"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"

	abci "github.com/cometbft/cometbft/abci/types"

	dbm "github.com/cosmos/cosmos-db"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/blockstm"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	runtimeservices "github.com/cosmos/cosmos-sdk/runtime/services"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	"github.com/cosmos/cosmos-sdk/x/auth/posthandler"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/consensus"
	consensuskeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	consensustypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	"github.com/cosmos/cosmos-sdk/x/distribution"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	evmserver "github.com/cosmos/evm/server"
	callbackkeeper "github.com/cosmos/evm/x/ibc/callbacks/keeper"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"

	ibc "github.com/cosmos/ibc-go/v10/modules/core"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"

	evmante "github.com/cosmos/evm/ante"
	antetypes "github.com/cosmos/evm/ante/types"
	evmmempool "github.com/cosmos/evm/mempool"
	srvflags "github.com/cosmos/evm/server/flags"
)

// fullPrecompilesApp is a test-only app with ERC20 and bank precompile wired.
type fullPrecompilesApp struct {
	*baseapp.BaseApp
	legacyAmino *codec.LegacyAmino
	appCodec    codec.Codec
	txConfig    client.TxConfig
	clientCtx   client.Context
	interfaceRegistry codectypes.InterfaceRegistry
	keys        map[string]*storetypes.KVStoreKey

	pendingTxListeners []evmante.PendingTxListener

	AccountKeeper         authkeeper.AccountKeeper
	BankKeeper            bankkeeper.Keeper
	StakingKeeper         *stakingkeeper.Keeper
	SlashingKeeper        slashingkeeper.Keeper
	DistributionKeeper    distrkeeper.Keeper
	GovKeeper             govkeeper.Keeper
	IBCKeeper             *ibckeeper.Keeper
	UpgradeKeeper         *upgradekeeper.Keeper
	ConsensusParamsKeeper consensuskeeper.Keeper

	EVMKeeper      *evmkeeper.Keeper
	FeeMarketKeeper feemarketkeeper.Keeper
	Erc20Keeper    erc20keeper.Keeper
	TransferKeeper transferkeeper.Keeper
	EVMMempool     *evmmempool.ExperimentalEVMMempool

	ModuleManager      *module.Manager
	BasicModuleManager module.BasicManager
	configurator       module.Configurator
}

var (
	_ runtime.AppI            = (*fullPrecompilesApp)(nil)
	_ servertypes.Application = (*fullPrecompilesApp)(nil)
	_ evm.EvmApp              = (*fullPrecompilesApp)(nil)
)

func createFullPrecompiles(chainID string, evmChainID uint64, opts ...func(*baseapp.BaseApp)) evm.EvmApp {
	defaultNodeHome, err := os.MkdirTemp("", "evmd-temp-homedir")
	if err != nil {
		panic(err)
	}

	db := dbm.NewMemDB()
	logger := log.NewNopLogger()
	loadLatest := true
	appOpts := integration.NewAppOptionsWithFlagHomeAndChainID(defaultNodeHome, evmChainID)
	encodingConfig := evmencoding.MakeConfig(evmChainID)

	appCodec := encodingConfig.Codec
	legacyAmino := encodingConfig.Amino
	interfaceRegistry := encodingConfig.InterfaceRegistry
	txConfig := encodingConfig.TxConfig

	baseAppOptions := append(opts, baseapp.SetChainID(chainID))

	bApp := baseapp.NewBaseApp("evmd", logger, db, txConfig.TxDecoder(), baseAppOptions...)
	bApp.SetCommitMultiStoreTracer(nil)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)
	bApp.SetTxEncoder(txConfig.TxEncoder())

	keys := storetypes.NewKVStoreKeys(
		authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
		distrtypes.StoreKey, slashingtypes.StoreKey, govtypes.StoreKey,
		upgradetypes.StoreKey, consensustypes.StoreKey, ibcexported.StoreKey,
		evmtypes.StoreKey, feemarkettypes.StoreKey, erc20types.StoreKey,
	)
	oKeys := storetypes.NewObjectStoreKeys(banktypes.ObjectStoreKey, evmtypes.ObjectKey)

	nonTransientKeys := make([]storetypes.StoreKey, 0, len(keys)+len(oKeys))
	for _, k := range keys {
		nonTransientKeys = append(nonTransientKeys, k)
	}
	for _, k := range oKeys {
		nonTransientKeys = append(nonTransientKeys, k)
	}

	bApp.SetBlockSTMTxRunner(blockstm.NewSTMRunner(
		encodingConfig.TxConfig.TxDecoder(),
		nonTransientKeys,
		min(goruntime.GOMAXPROCS(0), goruntime.NumCPU()),
		true,
		"astake",
	))

	if err := bApp.RegisterStreamingServices(appOpts, keys); err != nil {
		panic(err)
	}

	app := &fullPrecompilesApp{
		BaseApp:     bApp,
		legacyAmino: legacyAmino,
		appCodec:    appCodec,
		txConfig:    txConfig,
		interfaceRegistry: interfaceRegistry,
		keys:        keys,
	}

	app.SetDisableBlockGasMeter(true)

	authAddr := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	app.ConsensusParamsKeeper = consensuskeeper.NewKeeper(appCodec, runtime.NewKVStoreService(keys[consensustypes.StoreKey]), authAddr, runtime.EventService{})
	bApp.SetParamStore(app.ConsensusParamsKeeper.ParamsStore)

	app.AccountKeeper = authkeeper.NewAccountKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[authtypes.StoreKey]),
		authtypes.ProtoBaseAccount,
		maccPermsWithErc20(),
		address.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
		sdk.GetConfig().GetBech32AccountAddrPrefix(),
		authAddr,
	)

	app.BankKeeper = bankkeeper.NewBaseKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[banktypes.StoreKey]),
		app.AccountKeeper,
		evmconfig.BlockedAddresses(),
		authAddr,
		logger,
	)
	app.BankKeeper = app.BankKeeper.WithObjStoreKey(oKeys[banktypes.ObjectStoreKey])

	app.StakingKeeper = stakingkeeper.NewKeeper(
		appCodec, runtime.NewKVStoreService(keys[stakingtypes.StoreKey]), app.AccountKeeper, app.BankKeeper, authAddr, address.NewBech32Codec(sdk.GetConfig().GetBech32ValidatorAddrPrefix()), address.NewBech32Codec(sdk.GetConfig().GetBech32ConsensusAddrPrefix()),
	)

	app.DistributionKeeper = distrkeeper.NewKeeper(appCodec, runtime.NewKVStoreService(keys[distrtypes.StoreKey]), app.AccountKeeper, app.BankKeeper, app.StakingKeeper, authtypes.FeeCollectorName, authAddr)
	app.SlashingKeeper = slashingkeeper.NewKeeper(appCodec, app.LegacyAmino(), runtime.NewKVStoreService(keys[slashingtypes.StoreKey]), app.StakingKeeper, authAddr)
	app.StakingKeeper.SetHooks(
		stakingtypes.NewMultiStakingHooks(app.DistributionKeeper.Hooks(), app.SlashingKeeper.Hooks()),
	)

	skipUpgradeHeights := map[int64]bool{}
	for _, h := range cast.ToIntSlice(appOpts.Get(server.FlagUnsafeSkipUpgrades)) {
		skipUpgradeHeights[int64(h)] = true
	}
	homePath := cast.ToString(appOpts.Get(flags.FlagHome))
	app.UpgradeKeeper = upgradekeeper.NewKeeper(skipUpgradeHeights, runtime.NewKVStoreService(keys[upgradetypes.StoreKey]), appCodec, homePath, app.BaseApp, authAddr)

	app.IBCKeeper = ibckeeper.NewKeeper(
		appCodec, runtime.NewKVStoreService(keys[ibcexported.StoreKey]), app.UpgradeKeeper, authAddr,
	)

	govConfig := govtypes.DefaultConfig()
	govKeeper := govkeeper.NewKeeper(
		appCodec, runtime.NewKVStoreService(keys[govtypes.StoreKey]), app.AccountKeeper, app.BankKeeper,
		app.StakingKeeper, app.DistributionKeeper, app.MsgServiceRouter(), govConfig, authAddr,
	)
	app.GovKeeper = *govKeeper.SetHooks(
		govtypes.NewMultiGovHooks(),
	)

	app.FeeMarketKeeper = feemarketkeeper.NewKeeper(
		appCodec, authtypes.NewModuleAddress(govtypes.ModuleName),
		keys[feemarkettypes.StoreKey],
	)

	tracer := cast.ToString(appOpts.Get(srvflags.EVMTracer))
	app.EVMKeeper = evmkeeper.NewKeeper(
		appCodec,
		keys[evmtypes.StoreKey],
		oKeys[evmtypes.ObjectKey],
		nonTransientKeys,
		authtypes.NewModuleAddress(govtypes.ModuleName),
		app.AccountKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		app.FeeMarketKeeper,
		&app.ConsensusParamsKeeper,
		&app.Erc20Keeper,
		evmChainID,
		tracer,
	).WithStaticPrecompiles(
		precompilestypes.DefaultStaticPrecompiles(
			*app.StakingKeeper,
			app.DistributionKeeper,
			app.BankKeeper,
			&app.Erc20Keeper,
			nil,
			nil,
			app.IBCKeeper.ClientKeeper,
			app.GovKeeper,
			app.SlashingKeeper,
			app.AppCodec(),
		),
	)
	app.EVMKeeper.EnableVirtualFeeCollection()

	app.Erc20Keeper = erc20keeper.NewKeeper(
		keys[erc20types.StoreKey],
		app.AppCodec(),
		authtypes.NewModuleAddress(govtypes.ModuleName),
		app.GetAccountKeeper(),
		app.GetBankKeeper(),
		app.GetEVMKeeper(),
		app.GetStakingKeeper(),
		&app.TransferKeeper,
	)

	clientKeeper := app.IBCKeeper.ClientKeeper
	storeProvider := clientKeeper.GetStoreProvider()
	tmLightClientModule := ibctm.NewLightClientModule(appCodec, storeProvider)
	clientKeeper.AddRoute(ibctm.ModuleName, &tmLightClientModule)

	app.ModuleManager = module.NewManager(
		genutil.NewAppModule(
			app.AccountKeeper, app.StakingKeeper, app,
			txConfig,
		),
		auth.NewAppModule(appCodec, app.AccountKeeper, nil, nil),
		bank.NewAppModule(appCodec, app.BankKeeper, app.AccountKeeper, nil),
		gov.NewAppModule(appCodec, &app.GovKeeper, app.AccountKeeper, app.BankKeeper, nil),
		slashing.NewAppModule(appCodec, app.SlashingKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, nil, app.interfaceRegistry),
		distribution.NewAppModule(appCodec, app.DistributionKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, nil),
		staking.NewAppModule(appCodec, app.StakingKeeper, app.AccountKeeper, app.BankKeeper, nil),
		upgrade.NewAppModule(app.UpgradeKeeper, app.AccountKeeper.AddressCodec()),
		consensus.NewAppModule(appCodec, app.ConsensusParamsKeeper),
		ibc.NewAppModule(app.IBCKeeper),
		ibctm.NewAppModule(tmLightClientModule),
		vm.NewAppModule(app.EVMKeeper, app.AccountKeeper, app.BankKeeper, app.AccountKeeper.AddressCodec()),
		feemarket.NewAppModule(app.FeeMarketKeeper),
		erc20.NewAppModule(app.Erc20Keeper, app.AccountKeeper),
	)

	app.BasicModuleManager = module.NewBasicManagerFromManager(
		app.ModuleManager,
		map[string]module.AppModuleBasic{
			genutiltypes.ModuleName: genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
			govtypes.ModuleName:     gov.NewAppModuleBasic([]govclient.ProposalHandler{}),
		})
	app.BasicModuleManager.RegisterLegacyAminoCodec(legacyAmino)
	app.BasicModuleManager.RegisterInterfaces(interfaceRegistry)

	app.ModuleManager.SetOrderPreBlockers(
		upgradetypes.ModuleName,
		evmtypes.ModuleName,
		authtypes.ModuleName,
	)
	app.ModuleManager.SetOrderBeginBlockers(
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		stakingtypes.ModuleName,
		genutiltypes.ModuleName,
		ibcexported.ModuleName,
		feemarkettypes.ModuleName,
		evmtypes.ModuleName,
	)
	app.ModuleManager.SetOrderEndBlockers(
		banktypes.ModuleName,
		govtypes.ModuleName,
		stakingtypes.ModuleName,
		distrtypes.ModuleName,
		evmtypes.ModuleName,
		feemarkettypes.ModuleName,
	)

	genesisModuleOrder := []string{
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		stakingtypes.ModuleName,
		slashingtypes.ModuleName,
		govtypes.ModuleName,
		ibcexported.ModuleName,
		evmtypes.ModuleName,
		feemarkettypes.ModuleName,
		erc20types.ModuleName,
		genutiltypes.ModuleName,
		upgradetypes.ModuleName,
		consensustypes.ModuleName,
	}
	app.ModuleManager.SetOrderInitGenesis(genesisModuleOrder...)
	app.ModuleManager.SetOrderExportGenesis(genesisModuleOrder...)

	app.configurator = module.NewConfigurator(app.appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	if err := app.ModuleManager.RegisterServices(app.configurator); err != nil {
		panic(err)
	}

	app.RegisterUpgradeHandlers()

	autocliv1.RegisterQueryServer(app.GRPCQueryRouter(), runtimeservices.NewAutoCLIQueryService(app.ModuleManager.Modules))

	reflectionSvc, err := runtimeservices.NewReflectionService()
	if err != nil {
		panic(err)
	}
	reflectionv1.RegisterReflectionServiceServer(app.GRPCQueryRouter(), reflectionSvc)

	app.MountKVStores(keys)
	app.MountObjectStores(oKeys)

	maxGasWanted := cast.ToUint64(appOpts.Get(srvflags.EVMMaxTxGasWanted))
	app.SetInitChainer(app.InitChainer)
	app.SetPreBlocker(app.PreBlocker)
	app.SetBeginBlocker(app.BeginBlocker)
	app.SetEndBlocker(app.EndBlocker)
	app.setAnteHandler(app.txConfig, maxGasWanted)
	if err := app.configureEVMMempool(appOpts, logger); err != nil {
		panic(err)
	}
	app.setPostHandler()

	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			panic(fmt.Errorf("error loading last version: %w", err))
		}
	}

	return app
}

func maccPermsWithErc20() map[string][]string {
	return map[string][]string{
		authtypes.FeeCollectorName:     nil,
		distrtypes.ModuleName:          nil,
		minttypes.ModuleName:           {authtypes.Minter},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		govtypes.ModuleName:            {authtypes.Burner},
		evmtypes.ModuleName:            {authtypes.Minter, authtypes.Burner},
		feemarkettypes.ModuleName:      nil,
		erc20types.ModuleName:          {authtypes.Minter, authtypes.Burner},
	}
}

func (app *fullPrecompilesApp) setAnteHandler(txConfig client.TxConfig, maxGasWanted uint64) {
	options := evmante.HandlerOptions{
		Cdc:                    app.appCodec,
		AccountKeeper:          app.AccountKeeper,
		BankKeeper:             app.BankKeeper,
		ExtensionOptionChecker: antetypes.HasDynamicFeeExtensionOption,
		EvmKeeper:              app.EVMKeeper,
		FeeMarketKeeper:        app.FeeMarketKeeper,
		SignModeHandler:        txConfig.SignModeHandler(),
		SigGasConsumer:         evmante.SigVerificationGasConsumer,
		MaxTxGasWanted:         maxGasWanted,
		DynamicFeeChecker:      true,
		PendingTxListener:      app.onPendingTx,
		IBCKeeper:              app.IBCKeeper,
		FeegrantKeeper:         nil,
	}
	if err := validateAnteHandlerOptions(options); err != nil {
		panic(err)
	}
	app.SetAnteHandler(evmante.NewAnteHandler(options))
}

func (app *fullPrecompilesApp) setPostHandler() {
	postHandler, err := posthandler.NewPostHandler(
		posthandler.HandlerOptions{},
	)
	if err != nil {
		panic(err)
	}
	app.SetPostHandler(postHandler)
}

func (app *fullPrecompilesApp) onPendingTx(hash common.Hash) {
	for _, listener := range app.pendingTxListeners {
		listener(hash)
	}
}

func (app *fullPrecompilesApp) configureEVMMempool(appOpts servertypes.AppOptions, logger log.Logger) error {
	if evmtypes.GetChainConfig() == nil {
		logger.Debug("evm chain config is not set, skipping mempool configuration")
		return nil
	}

	cosmosPoolMaxTx := evmserver.GetCosmosPoolMaxTx(appOpts, logger)
	if cosmosPoolMaxTx < 0 {
		cosmosPoolMaxTx = 0
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
	app.SetPrepareProposal(abciProposalHandler.PrepareProposalHandler())

	return nil
}

func (app *fullPrecompilesApp) createMempoolConfig(appOpts servertypes.AppOptions, logger log.Logger) (*evmmempool.EVMMempoolConfig, error) {
	return &evmmempool.EVMMempoolConfig{
		AnteHandler:      app.GetAnteHandler(),
		LegacyPoolConfig: evmserver.GetLegacyPoolConfig(appOpts, logger),
		BlockGasLimit:    evmserver.GetBlockGasLimit(appOpts, logger),
		MinTip:           evmserver.GetMinTip(appOpts, logger),
	}, nil
}

// ---- interface methods ----

func (app *fullPrecompilesApp) LegacyAmino() *codec.LegacyAmino { return app.legacyAmino }
func (app *fullPrecompilesApp) AppCodec() codec.Codec           { return app.appCodec }
func (app *fullPrecompilesApp) InterfaceRegistry() codectypes.InterfaceRegistry {
	return app.interfaceRegistry
}
func (app *fullPrecompilesApp) GetTxConfig() client.TxConfig { return app.txConfig }
func (app *fullPrecompilesApp) GetKey(storeKey string) *storetypes.KVStoreKey {
	return app.keys[storeKey]
}
func (app *fullPrecompilesApp) DefaultGenesis() map[string]json.RawMessage {
	genesis := app.BasicModuleManager.DefaultGenesis(app.appCodec)
	evmGenState := evmtypes.DefaultGenesisState()
	evmGenState.Params.ActiveStaticPrecompiles = evmtypes.AvailableStaticPrecompiles
	evmGenState.Preinstalls = evmtypes.DefaultPreinstalls
	genesis[evmtypes.ModuleName] = app.appCodec.MustMarshalJSON(evmGenState)
	genesis[erc20types.ModuleName] = app.appCodec.MustMarshalJSON(erc20types.DefaultGenesisState())
	return genesis
}
func (app *fullPrecompilesApp) LoadHeight(height int64) error { return app.LoadVersion(height) }

// ExportAppStateAndValidators provides a minimal implementation for tests.
func (app *fullPrecompilesApp) ExportAppStateAndValidators(forZeroHeight bool, jailAllowedAddrs []string, modulesToExport []string) (servertypes.ExportedApp, error) {
	ctx := app.NewContext(true)
	genState, err := app.ModuleManager.ExportGenesis(ctx, app.appCodec)
	if err != nil {
		return servertypes.ExportedApp{}, err
	}
	appState, err := json.MarshalIndent(genState, "", "  ")
	if err != nil {
		return servertypes.ExportedApp{}, err
	}
	return servertypes.ExportedApp{
		AppState:   appState,
		Validators: []cmttypes.GenesisValidator{},
		Height:     app.LastBlockHeight() + 1,
	}, nil
}

// servertypes.Application stubs
func (app *fullPrecompilesApp) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {}
func (app *fullPrecompilesApp) RegisterTxService(clientCtx client.Context)                      {}
func (app *fullPrecompilesApp) RegisterTendermintService(clientCtx client.Context)             {}
func (app *fullPrecompilesApp) RegisterNodeService(clientCtx client.Context, cfg config.Config) {}
func (app *fullPrecompilesApp) PreBlocker(ctx sdk.Context, _ *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
	return app.ModuleManager.PreBlock(ctx)
}
func (app *fullPrecompilesApp) BeginBlocker(ctx sdk.Context) (sdk.BeginBlock, error) {
	return app.ModuleManager.BeginBlock(ctx)
}
func (app *fullPrecompilesApp) EndBlocker(ctx sdk.Context) (sdk.EndBlock, error) {
	return app.ModuleManager.EndBlock(ctx)
}
func (app *fullPrecompilesApp) InitChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var genesisState map[string]json.RawMessage
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}
	if err := app.UpgradeKeeper.SetModuleVersionMap(ctx, app.ModuleManager.GetVersionMap()); err != nil {
		panic(err)
	}
	return app.ModuleManager.InitGenesis(ctx, app.appCodec, genesisState)
}

// evm.EvmApp getters (many unused in bank tests return zero values)
func (app *fullPrecompilesApp) GetEVMKeeper() *evmkeeper.Keeper              { return app.EVMKeeper }
func (app *fullPrecompilesApp) GetErc20Keeper() *erc20keeper.Keeper          { return &app.Erc20Keeper }
func (app *fullPrecompilesApp) SetErc20Keeper(k erc20keeper.Keeper)          { app.Erc20Keeper = k }
func (app *fullPrecompilesApp) GetGovKeeper() govkeeper.Keeper               { return app.GovKeeper }
func (app *fullPrecompilesApp) GetFeeMarketKeeper() *feemarketkeeper.Keeper  { return &app.FeeMarketKeeper }
func (app *fullPrecompilesApp) GetBankKeeper() bankkeeper.Keeper             { return app.BankKeeper }
func (app *fullPrecompilesApp) GetAccountKeeper() authkeeper.AccountKeeper   { return app.AccountKeeper }
func (app *fullPrecompilesApp) GetStakingKeeper() *stakingkeeper.Keeper      { return app.StakingKeeper }
func (app *fullPrecompilesApp) GetIBCKeeper() *ibckeeper.Keeper              { return app.IBCKeeper }
func (app *fullPrecompilesApp) GetMempool() sdkmempool.ExtMempool            { return app.EVMMempool }
func (app *fullPrecompilesApp) GetAnteHandler() sdk.AnteHandler              { return app.AnteHandler() }
func (app *fullPrecompilesApp) GetConsensusParamsKeeper() consensuskeeper.Keeper {
	return app.ConsensusParamsKeeper
}
func (app *fullPrecompilesApp) GetDistrKeeper() distrkeeper.Keeper           { return app.DistributionKeeper }
func (app *fullPrecompilesApp) GetSlashingKeeper() slashingkeeper.Keeper    { return app.SlashingKeeper }
func (app *fullPrecompilesApp) GetEvidenceKeeper() *evidencekeeper.Keeper   { return nil }
func (app *fullPrecompilesApp) GetAuthzKeeper() authzkeeper.Keeper          { return authzkeeper.Keeper{} }
func (app *fullPrecompilesApp) GetMintKeeper() mintkeeper.Keeper            { return mintkeeper.Keeper{} }
func (app *fullPrecompilesApp) GetPreciseBankKeeper() *precisebankkeeper.Keeper {
	return nil
}
func (app *fullPrecompilesApp) GetFeeGrantKeeper() feegrantkeeper.Keeper    { return feegrantkeeper.Keeper{} }
func (app *fullPrecompilesApp) GetCallbackKeeper() callbackkeeper.ContractKeeper {
	return callbackkeeper.ContractKeeper{}
}
func (app *fullPrecompilesApp) GetTransferKeeper() transferkeeper.Keeper   { return app.TransferKeeper }
func (app *fullPrecompilesApp) SetTransferKeeper(transferKeeper transferkeeper.Keeper) {
	app.TransferKeeper = transferKeeper
}
func (app *fullPrecompilesApp) SimulationManager() *module.SimulationManager          { return nil }

// RegisterUpgradeHandlers is a no-op for the test app.
func (app *fullPrecompilesApp) RegisterUpgradeHandlers() {}

// helper for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Copy of validateAnteHandlerOptions from evmd/app
func validateAnteHandlerOptions(options evmante.HandlerOptions) error {
	if options.AccountKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "account keeper is required for AnteHandler")
	}
	if options.BankKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "bank keeper is required for AnteHandler")
	}
	if options.SignModeHandler == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "sign mode handler is required for AnteHandler")
	}
	if options.FeeMarketKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "feemarket keeper is required for AnteHandler")
	}
	if options.IBCKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "IBC keeper is required for AnteHandler")
	}
	return nil
}
