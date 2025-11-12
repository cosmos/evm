package evmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cast"

	// Force-load the tracer engines to trigger registration due to Go-Ethereum v1.10.15 changes
	"github.com/ethereum/go-ethereum/common"
	_ "github.com/ethereum/go-ethereum/eth/tracers/js"
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"

	abci "github.com/cometbft/cometbft/abci/types"

	dbm "github.com/cosmos/cosmos-db"
	evmante "github.com/cosmos/evm/ante"
	antetypes "github.com/cosmos/evm/ante/types"
	evmencoding "github.com/cosmos/evm/encoding"
	evmaddress "github.com/cosmos/evm/encoding/address"
	evmconfig "github.com/cosmos/evm/evmd/config"
	evmmempool "github.com/cosmos/evm/mempool"
	precompiletypes "github.com/cosmos/evm/precompiles/types"
	srvflags "github.com/cosmos/evm/server/flags"
	"github.com/cosmos/evm/utils"
	"github.com/cosmos/evm/x/erc20"
	erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	erc20v2 "github.com/cosmos/evm/x/erc20/v2"
	"github.com/cosmos/evm/x/feemarket"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	ibccallbackskeeper "github.com/cosmos/evm/x/ibc/callbacks/keeper"

	"github.com/cosmos/evm/x/ibc/transfer"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
	transferv2 "github.com/cosmos/evm/x/ibc/transfer/v2"
	"github.com/cosmos/evm/x/precisebank"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	"github.com/cosmos/evm/x/vm"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/gogoproto/proto"
	ibccallbacks "github.com/cosmos/ibc-go/v10/modules/apps/callbacks"
	ibctransfer "github.com/cosmos/ibc-go/v10/modules/apps/transfer"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	ibc "github.com/cosmos/ibc-go/v10/modules/core"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcapi "github.com/cosmos/ibc-go/v10/modules/core/api"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
	reflectionv1 "cosmossdk.io/api/cosmos/reflection/v1"
	"cosmossdk.io/client/v2/autocli"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/evidence"
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"
	feegrantmodule "cosmossdk.io/x/feegrant/module"
	"cosmossdk.io/x/upgrade"
	upgradekeeper "cosmossdk.io/x/upgrade/keeper"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	"github.com/cosmos/cosmos-sdk/client/grpc/node"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	runtimeservices "github.com/cosmos/cosmos-sdk/runtime/services"
	sdkserver "github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	testdata_pulsar "github.com/cosmos/cosmos-sdk/testutil/testdata/testpb"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/version"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	"github.com/cosmos/cosmos-sdk/x/auth/posthandler"
	authsims "github.com/cosmos/cosmos-sdk/x/auth/simulation"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	txmodule "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/auth/vesting"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	authzmodule "github.com/cosmos/cosmos-sdk/x/authz/module"
	"github.com/cosmos/cosmos-sdk/x/bank"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/consensus"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	distr "github.com/cosmos/cosmos-sdk/x/distribution"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/mint"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	cosmosevmserver "github.com/cosmos/evm/server"
)

func init() {
	// manually update the power reduction by replacing micro (u) -> atto (a) evmos
	sdk.DefaultPowerReduction = utils.AttoPowerReduction

	defaultNodeHome = evmconfig.MustGetDefaultNodeHome()
}

const appName = "evmd"

// defaultNodeHome default home directories for the application daemon
var defaultNodeHome string

var (
	_ runtime.AppI                = (*EVMD)(nil)
	_ cosmosevmserver.Application = (*EVMD)(nil)
	_ ibctesting.TestingApp       = (*EVMD)(nil)
)

// EVMD extends an ABCI application, but with most of its parameters exported.
type EVMD struct {
	*baseapp.BaseApp

	legacyAmino       *codec.LegacyAmino
	appCodec          codec.Codec
	interfaceRegistry types.InterfaceRegistry
	txConfig          client.TxConfig
	clientCtx         client.Context

	pendingTxListeners []evmante.PendingTxListener

	// keys to access the substores
	keys  map[string]*storetypes.KVStoreKey
	oKeys map[string]*storetypes.ObjectStoreKey

	// keepers
	AccountKeeper         authkeeper.AccountKeeper
	BankKeeper            bankkeeper.Keeper
	StakingKeeper         *stakingkeeper.Keeper
	SlashingKeeper        slashingkeeper.Keeper
	MintKeeper            mintkeeper.Keeper
	DistrKeeper           distrkeeper.Keeper
	GovKeeper             govkeeper.Keeper
	UpgradeKeeper         *upgradekeeper.Keeper
	AuthzKeeper           authzkeeper.Keeper
	EvidenceKeeper        evidencekeeper.Keeper
	FeeGrantKeeper        feegrantkeeper.Keeper
	ConsensusParamsKeeper consensusparamkeeper.Keeper

	// IBC keepers
	IBCKeeper      *ibckeeper.Keeper // IBC Keeper must be a pointer in the app, so we can SetRouter on it correctly
	TransferKeeper transferkeeper.Keeper
	CallbackKeeper ibccallbackskeeper.ContractKeeper

	// Cosmos EVM keepers
	FeeMarketKeeper   feemarketkeeper.Keeper
	EVMKeeper         *evmkeeper.Keeper
	Erc20Keeper       erc20keeper.Keeper
	PreciseBankKeeper precisebankkeeper.Keeper
	EVMMempool        *evmmempool.ExperimentalEVMMempool

	// the module manager
	ModuleManager      *module.Manager
	BasicModuleManager module.BasicManager

	// simulation manager
	sm *module.SimulationManager

	// module configurator
	configurator module.Configurator
}

// NewExampleApp returns a reference to an initialized EVMD.
func NewExampleApp(
	logger log.Logger,
	db dbm.DB,
	traceStore io.Writer,
	loadLatest bool,
	appOpts servertypes.AppOptions,
	baseAppOptions ...func(*baseapp.BaseApp),
) *EVMD {
	evmChainID := cast.ToUint64(appOpts.Get(srvflags.EVMChainID))
	encodingConfig := evmencoding.MakeConfig(evmChainID)

	appCodec := encodingConfig.Codec
	legacyAmino := encodingConfig.Amino
	interfaceRegistry := encodingConfig.InterfaceRegistry
	txConfig := encodingConfig.TxConfig

	bApp := baseapp.NewBaseApp(
		appName,
		logger,
		db,
		// use transaction decoder to support the sdk.Tx interface instead of sdk.StdTx
		encodingConfig.TxConfig.TxDecoder(),
		baseAppOptions...,
	)
	bApp.SetCommitMultiStoreTracer(traceStore)
	bApp.SetVersion(version.Version)
	bApp.SetInterfaceRegistry(interfaceRegistry)
	bApp.SetTxEncoder(txConfig.TxEncoder())

	keys := storetypes.NewKVStoreKeys(
		authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
		minttypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey,
		govtypes.StoreKey, consensusparamtypes.StoreKey,
		upgradetypes.StoreKey, feegrant.StoreKey, evidencetypes.StoreKey, authzkeeper.StoreKey,
		// ibc keys
		ibcexported.StoreKey, ibctransfertypes.StoreKey,
		// Cosmos EVM store keys
		evmtypes.StoreKey, feemarkettypes.StoreKey, erc20types.StoreKey, precisebanktypes.StoreKey,
	)
	oKeys := storetypes.NewObjectStoreKeys(banktypes.ObjectStoreKey, evmtypes.ObjectKey)

	var nonTransientKeys []storetypes.StoreKey
	for _, k := range keys {
		nonTransientKeys = append(nonTransientKeys, k)
	}
	for _, k := range oKeys {
		nonTransientKeys = append(nonTransientKeys, k)
	}

	// load state streaming if enabled
	if err := bApp.RegisterStreamingServices(appOpts, keys); err != nil {
		fmt.Printf("failed to load state streaming: %s", err)
		os.Exit(1)
	}

	// wire up the versiondb's `StreamingService` and `MultiStore`.
	if cast.ToBool(appOpts.Get("versiondb.enable")) {
		panic("version db not supported in this example chain")
	}

	app := &EVMD{
		BaseApp:           bApp,
		legacyAmino:       legacyAmino,
		appCodec:          appCodec,
		txConfig:          txConfig,
		interfaceRegistry: interfaceRegistry,
		keys:              keys,
		oKeys:             oKeys,
	}

	// removed x/params: no ParamsKeeper initialization

	// get authority address
	authAddr := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	// set the BaseApp's parameter store
	app.ConsensusParamsKeeper = consensusparamkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[consensusparamtypes.StoreKey]),
		authAddr,
		runtime.EventService{},
	)
	bApp.SetParamStore(app.ConsensusParamsKeeper.ParamsStore)

	// add keepers
	app.AccountKeeper = authkeeper.NewAccountKeeper(
		appCodec, runtime.NewKVStoreService(keys[authtypes.StoreKey]),
		authtypes.ProtoBaseAccount, evmconfig.GetMaccPerms(),
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
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

	// optional: enable sign mode textual by overwriting the default tx config (after setting the bank keeper)
	enabledSignModes := append(authtx.DefaultSignModes, signingtypes.SignMode_SIGN_MODE_TEXTUAL) //nolint:gocritic
	txConfigOpts := authtx.ConfigOptions{
		EnabledSignModes:           enabledSignModes,
		TextualCoinMetadataQueryFn: txmodule.NewBankKeeperCoinMetadataQueryFn(app.BankKeeper),
	}
	txConfig, err := authtx.NewTxConfigWithOptions(
		appCodec,
		txConfigOpts,
	)
	if err != nil {
		panic(err)
	}
	app.txConfig = txConfig

	app.StakingKeeper = stakingkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[stakingtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		authAddr,
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32ValidatorAddrPrefix()),
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32ConsensusAddrPrefix()),
	)

	app.MintKeeper = mintkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[minttypes.StoreKey]),
		app.StakingKeeper,
		app.AccountKeeper,
		app.BankKeeper,
		authtypes.FeeCollectorName,
		authAddr,
	)

	app.DistrKeeper = distrkeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[distrtypes.StoreKey]),
		app.AccountKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		authtypes.FeeCollectorName,
		authAddr,
	)

	app.SlashingKeeper = slashingkeeper.NewKeeper(
		appCodec,
		app.LegacyAmino(),
		runtime.NewKVStoreService(keys[slashingtypes.StoreKey]),
		app.StakingKeeper,
		authAddr,
	)

	app.FeeGrantKeeper = feegrantkeeper.NewKeeper(appCodec, runtime.NewKVStoreService(keys[feegrant.StoreKey]), app.AccountKeeper)

	// register the staking hooks
	// NOTE: stakingKeeper above is passed by reference, so that it will contain these hooks
	app.StakingKeeper.SetHooks(
		stakingtypes.NewMultiStakingHooks(app.DistrKeeper.Hooks(), app.SlashingKeeper.Hooks()),
	)

	app.AuthzKeeper = authzkeeper.NewKeeper(
		runtime.NewKVStoreService(keys[authzkeeper.StoreKey]),
		appCodec,
		app.MsgServiceRouter(),
		app.AccountKeeper,
	)

	// get skipUpgradeHeights from the app options
	skipUpgradeHeights := map[int64]bool{}
	for _, h := range cast.ToIntSlice(appOpts.Get(sdkserver.FlagUnsafeSkipUpgrades)) {
		skipUpgradeHeights[int64(h)] = true
	}
	homePath := cast.ToString(appOpts.Get(flags.FlagHome))
	// set the governance module account as the authority for conducting upgrades
	app.UpgradeKeeper = upgradekeeper.NewKeeper(
		skipUpgradeHeights,
		runtime.NewKVStoreService(keys[upgradetypes.StoreKey]),
		appCodec,
		homePath,
		app.BaseApp,
		authAddr,
	)

	// Create IBC Keeper
	app.IBCKeeper = ibckeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[ibcexported.StoreKey]),
		app.UpgradeKeeper,
		authAddr,
	)

	govConfig := govtypes.DefaultConfig()
	/*
		Example of setting gov params:
		govConfig.MaxMetadataLen = 10000
	*/
	govKeeper := govkeeper.NewKeeper(
		appCodec, runtime.NewKVStoreService(keys[govtypes.StoreKey]), app.AccountKeeper, app.BankKeeper,
		app.StakingKeeper, app.DistrKeeper, app.MsgServiceRouter(), govConfig, authAddr,
	)

	app.GovKeeper = *govKeeper.SetHooks(
		govtypes.NewMultiGovHooks(
		// register the governance hooks
		),
	)

	// create evidence keeper with router
	evidenceKeeper := evidencekeeper.NewKeeper(
		appCodec,
		runtime.NewKVStoreService(keys[evidencetypes.StoreKey]),
		app.StakingKeeper,
		app.SlashingKeeper,
		app.AccountKeeper.AddressCodec(),
		runtime.ProvideCometInfoService(),
	)
	// If evidence needs to be handled for the app, set routes in router here and seal
	app.EvidenceKeeper = *evidenceKeeper

	// Cosmos EVM keepers
	app.FeeMarketKeeper = feemarketkeeper.NewKeeper(
		appCodec, authtypes.NewModuleAddress(govtypes.ModuleName),
		keys[feemarkettypes.StoreKey],
	)

	// Set up PreciseBank keeper
	//
	// NOTE: PreciseBank is not needed if SDK use 18 decimals for gas coin. Use BankKeeper instead.
	app.PreciseBankKeeper = precisebankkeeper.NewKeeper(
		appCodec,
		keys[precisebanktypes.StoreKey],
		app.BankKeeper,
		app.AccountKeeper,
	)

	// Set up EVM keeper
	tracer := cast.ToString(appOpts.Get(srvflags.EVMTracer))

	// NOTE: it's required to set up the EVM keeper before the ERC-20 keeper, because it is used in its instantiation.
	app.EVMKeeper = evmkeeper.NewKeeper(
		// TODO: check why this is not adjusted to use the runtime module methods like SDK native keepers
		appCodec, keys[evmtypes.StoreKey], oKeys[evmtypes.ObjectKey], nonTransientKeys,
		authtypes.NewModuleAddress(govtypes.ModuleName),
		app.AccountKeeper,
		app.PreciseBankKeeper,
		app.StakingKeeper,
		app.FeeMarketKeeper,
		&app.ConsensusParamsKeeper,
		&app.Erc20Keeper,
		evmChainID,
		tracer,
	).WithStaticPrecompiles(
		precompiletypes.DefaultStaticPrecompiles(
			*app.StakingKeeper,
			app.DistrKeeper,
			app.PreciseBankKeeper,
			&app.Erc20Keeper,
			&app.TransferKeeper,
			app.IBCKeeper.ChannelKeeper,
			app.IBCKeeper.ClientKeeper,
			app.GovKeeper,
			app.SlashingKeeper,
			appCodec,
		),
	)

	app.Erc20Keeper = erc20keeper.NewKeeper(
		keys[erc20types.StoreKey],
		appCodec,
		authtypes.NewModuleAddress(govtypes.ModuleName),
		app.AccountKeeper,
		app.PreciseBankKeeper,
		app.EVMKeeper,
		app.StakingKeeper,
		&app.TransferKeeper,
	)

	// instantiate IBC transfer keeper AFTER the ERC-20 keeper to use it in the instantiation
	app.TransferKeeper = transferkeeper.NewKeeper(
		appCodec,
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
		runtime.NewKVStoreService(keys[ibctransfertypes.StoreKey]),
		app.IBCKeeper.ChannelKeeper,
		app.MsgServiceRouter(),
		app.AccountKeeper,
		app.BankKeeper,
		app.Erc20Keeper, // Add ERC20 Keeper for ERC20 transfers
		authAddr,
	)

	/*
		Create Transfer Stack

		transfer stack contains (from bottom to top):
			- IBC Callbacks Middleware (with EVM ContractKeeper)
			- ERC-20 Middleware
			- IBC Transfer

		SendPacket, since it is originating from the application to core IBC:
		 	transferKeeper.SendPacket ->  erc20.SendPacket -> callbacks.SendPacket -> channel.SendPacket

		RecvPacket, message that originates from core IBC and goes down to app, the flow is the other way
			channel.RecvPacket -> callbacks.OnRecvPacket -> erc20.OnRecvPacket -> transfer.OnRecvPacket
	*/

	// create IBC module from top to bottom of stack
	var transferStack porttypes.IBCModule

	transferStack = transfer.NewIBCModule(app.TransferKeeper)
	maxCallbackGas := uint64(1_000_000)
	transferStack = erc20.NewIBCMiddleware(app.Erc20Keeper, transferStack)
	app.CallbackKeeper = ibccallbackskeeper.NewKeeper(
		app.AccountKeeper,
		app.EVMKeeper,
		app.Erc20Keeper,
	)
	callbacksMiddleware := ibccallbacks.NewIBCMiddleware(app.CallbackKeeper, maxCallbackGas)
	callbacksMiddleware.SetICS4Wrapper(app.IBCKeeper.ChannelKeeper)
	callbacksMiddleware.SetUnderlyingApplication(transferStack)
	transferStack = callbacksMiddleware

	var transferStackV2 ibcapi.IBCModule
	transferStackV2 = transferv2.NewIBCModule(app.TransferKeeper)
	transferStackV2 = erc20v2.NewIBCMiddleware(transferStackV2, app.Erc20Keeper)

	// Create static IBC router, add transfer route, then set and seal it
	ibcRouter := porttypes.NewRouter()
	ibcRouter.AddRoute(ibctransfertypes.ModuleName, transferStack)
	ibcRouterV2 := ibcapi.NewRouter()
	ibcRouterV2.AddRoute(ibctransfertypes.ModuleName, transferStackV2)

	app.IBCKeeper.SetRouter(ibcRouter)
	app.IBCKeeper.SetRouterV2(ibcRouterV2)

	clientKeeper := app.IBCKeeper.ClientKeeper
	storeProvider := app.IBCKeeper.ClientKeeper.GetStoreProvider()
	tmLightClientModule := ibctm.NewLightClientModule(appCodec, storeProvider)
	clientKeeper.AddRoute(ibctm.ModuleName, &tmLightClientModule)

	// Override the ICS20 app module
	transferModule := transfer.NewAppModule(app.TransferKeeper)

	/****  Module Options ****/

	// NOTE: Any module instantiated in the module manager that is later modified
	// must be passed by reference here.
	app.ModuleManager = module.NewManager(
		genutil.NewAppModule(
			app.AccountKeeper, app.StakingKeeper,
			app, app.txConfig,
		),
		auth.NewAppModule(appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, nil),
		bank.NewAppModule(appCodec, app.BankKeeper, app.AccountKeeper, nil),
		feegrantmodule.NewAppModule(appCodec, app.AccountKeeper, app.BankKeeper, app.FeeGrantKeeper, app.interfaceRegistry),
		gov.NewAppModule(appCodec, &app.GovKeeper, app.AccountKeeper, app.BankKeeper, nil),
		mint.NewAppModule(appCodec, app.MintKeeper, app.AccountKeeper, nil, nil),
		slashing.NewAppModule(appCodec, app.SlashingKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, nil, app.interfaceRegistry),
		distr.NewAppModule(appCodec, app.DistrKeeper, app.AccountKeeper, app.BankKeeper, app.StakingKeeper, nil),
		staking.NewAppModule(appCodec, app.StakingKeeper, app.AccountKeeper, app.BankKeeper, nil),
		upgrade.NewAppModule(app.UpgradeKeeper, app.AccountKeeper.AddressCodec()),
		evidence.NewAppModule(app.EvidenceKeeper),
		authzmodule.NewAppModule(appCodec, app.AuthzKeeper, app.AccountKeeper, app.BankKeeper, app.interfaceRegistry),
		consensus.NewAppModule(appCodec, app.ConsensusParamsKeeper),
		vesting.NewAppModule(app.AccountKeeper, app.BankKeeper),
		// IBC modules
		ibc.NewAppModule(app.IBCKeeper),
		ibctm.NewAppModule(tmLightClientModule),
		transferModule,
		// Cosmos EVM modules
		vm.NewAppModule(app.EVMKeeper, app.AccountKeeper, app.BankKeeper, app.AccountKeeper.AddressCodec()),
		feemarket.NewAppModule(app.FeeMarketKeeper),
		erc20.NewAppModule(app.Erc20Keeper, app.AccountKeeper),
		precisebank.NewAppModule(app.PreciseBankKeeper, app.BankKeeper, app.AccountKeeper),
	)

	// BasicModuleManager defines the module BasicManager which is in charge of setting up basic,
	// non-dependant module elements, such as codec registration and genesis verification.
	// By default, it is composed of all the modules from the module manager.
	// Additionally, app module basics can be overwritten by passing them as an argument.
	app.BasicModuleManager = module.NewBasicManagerFromManager(
		app.ModuleManager,
		map[string]module.AppModuleBasic{
			genutiltypes.ModuleName:     genutil.NewAppModuleBasic(genutiltypes.DefaultMessageValidator),
			stakingtypes.ModuleName:     staking.AppModuleBasic{},
			govtypes.ModuleName:         gov.NewAppModuleBasic(nil),
			ibctransfertypes.ModuleName: transfer.AppModuleBasic{AppModuleBasic: &ibctransfer.AppModuleBasic{}},
		},
	)
	app.BasicModuleManager.RegisterLegacyAminoCodec(legacyAmino)
	app.BasicModuleManager.RegisterInterfaces(interfaceRegistry)

	// NOTE: upgrade module is required to be prioritized
	app.ModuleManager.SetOrderPreBlockers(
		upgradetypes.ModuleName,
		authtypes.ModuleName,
		evmtypes.ModuleName,
	)

	// During begin block slashing happens after distr.BeginBlocker so that
	// there is nothing left over in the validator fee pool, so as to keep the
	// CanWithdrawInvariant invariant.
	//
	// NOTE: staking module is required if HistoricalEntries param > 0
	// NOTE: capability module's beginblocker must come before any modules using capabilities (e.g. IBC)
	app.ModuleManager.SetOrderBeginBlockers(
		minttypes.ModuleName,

		// IBC modules
		ibcexported.ModuleName, ibctransfertypes.ModuleName,

		// Cosmos EVM BeginBlockers
		erc20types.ModuleName, feemarkettypes.ModuleName,
		evmtypes.ModuleName, // NOTE: EVM BeginBlocker must come after FeeMarket BeginBlocker

		// TODO: remove no-ops? check if all are no-ops before removing
		distrtypes.ModuleName, slashingtypes.ModuleName,
		evidencetypes.ModuleName, stakingtypes.ModuleName,
		authtypes.ModuleName, banktypes.ModuleName, govtypes.ModuleName, genutiltypes.ModuleName,
		authz.ModuleName, feegrant.ModuleName,
		consensusparamtypes.ModuleName,
		precisebanktypes.ModuleName,
		vestingtypes.ModuleName,
	)

	// NOTE: the feemarket module should go last in order of end blockers that are actually doing something,
	// to get the full block gas used.
	app.ModuleManager.SetOrderEndBlockers(
		banktypes.ModuleName,
		govtypes.ModuleName,
		stakingtypes.ModuleName,
		authtypes.ModuleName,

		// Cosmos EVM EndBlockers
		evmtypes.ModuleName, erc20types.ModuleName, feemarkettypes.ModuleName,

		// no-ops
		ibcexported.ModuleName, ibctransfertypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName, minttypes.ModuleName,
		genutiltypes.ModuleName, evidencetypes.ModuleName, authz.ModuleName,
		feegrant.ModuleName, upgradetypes.ModuleName, consensusparamtypes.ModuleName,
		precisebanktypes.ModuleName,
		vestingtypes.ModuleName,
	)

	// NOTE: The genutils module must occur after staking so that pools are
	// properly initialized with tokens from genesis accounts.
	// NOTE: The genutils module must also occur after auth so that it can access the params from auth.
	genesisModuleOrder := []string{
		authtypes.ModuleName, banktypes.ModuleName,
		distrtypes.ModuleName, stakingtypes.ModuleName, slashingtypes.ModuleName, govtypes.ModuleName,
		minttypes.ModuleName,
		ibcexported.ModuleName,

		// Cosmos EVM modules
		//
		// NOTE: feemarket module needs to be initialized before genutil module:
		// gentx transactions use MinGasPriceDecorator.AnteHandle
		evmtypes.ModuleName,
		feemarkettypes.ModuleName,
		erc20types.ModuleName,
		precisebanktypes.ModuleName,

		ibctransfertypes.ModuleName,
		genutiltypes.ModuleName, evidencetypes.ModuleName, authz.ModuleName,
		feegrant.ModuleName, upgradetypes.ModuleName, vestingtypes.ModuleName,
	}
	app.ModuleManager.SetOrderInitGenesis(genesisModuleOrder...)
	app.ModuleManager.SetOrderExportGenesis(genesisModuleOrder...)

	// Uncomment if you want to set a custom migration order here.
	// app.ModuleManager.SetOrderMigrations(custom order)

	app.configurator = module.NewConfigurator(app.appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
	if err = app.ModuleManager.RegisterServices(app.configurator); err != nil {
		panic(fmt.Sprintf("failed to register services in module manager: %s", err.Error()))
	}

	// RegisterUpgradeHandlers is used for registering any on-chain upgrades.
	// Make sure it's called after `app.ModuleManager` and `app.configurator` are set.
	app.RegisterUpgradeHandlers()

	autocliv1.RegisterQueryServer(app.GRPCQueryRouter(), runtimeservices.NewAutoCLIQueryService(app.ModuleManager.Modules))

	reflectionSvc, err := runtimeservices.NewReflectionService()
	if err != nil {
		panic(err)
	}
	reflectionv1.RegisterReflectionServiceServer(app.GRPCQueryRouter(), reflectionSvc)

	// add test gRPC service for testing gRPC queries in isolation
	testdata_pulsar.RegisterQueryServer(app.GRPCQueryRouter(), testdata_pulsar.QueryImpl{})

	// create the simulation manager and define the order of the modules for deterministic simulations
	//
	// NOTE: this is not required apps that don't use the simulator for fuzz testing
	// transactions
	overrideModules := map[string]module.AppModuleSimulation{
		authtypes.ModuleName: auth.NewAppModule(app.appCodec, app.AccountKeeper, authsims.RandomGenesisAccounts, nil),
	}
	app.sm = module.NewSimulationManagerFromAppModules(app.ModuleManager.Modules, overrideModules)

	app.sm.RegisterStoreDecoders()

	// initialize stores
	app.MountKVStores(keys)
	app.MountObjectStores(oKeys)

	maxGasWanted := cast.ToUint64(appOpts.Get(srvflags.EVMMaxTxGasWanted))

	// initialize BaseApp
	app.SetInitChainer(app.InitChainer)
	app.SetPreBlocker(app.PreBlocker)
	app.SetBeginBlocker(app.BeginBlocker)
	app.SetEndBlocker(app.EndBlocker)

	app.setAnteHandler(app.txConfig, maxGasWanted)

	// set the EVM priority nonce mempool
	// if you wish to use the noop mempool, remove this codeblock
	if err := app.configureEVMMempool(appOpts, logger); err != nil {
		panic(fmt.Sprintf("failed to configure EVM mempool: %s", err.Error()))
	}

	// In v0.46, the SDK introduces _postHandlers_. PostHandlers are like
	// antehandlers, but are run _after_ the `runMsgs` execution. They are also
	// defined as a chain, and have the same signature as antehandlers.
	//
	// In baseapp, postHandlers are run in the same store branch as `runMsgs`,
	// meaning that both `runMsgs` and `postHandler` state will be committed if
	// both are successful, and both will be reverted if any of the two fails.
	//
	// The SDK exposes a default postHandlers chain, which comprises of only
	// one decorator: the Transaction Tips decorator. However, some chains do
	// not need it by default, so feel free to comment the next line if you do
	// not need tips.
	// To read more about tips:
	// https://docs.cosmos.network/main/core/tips.html
	//
	// Please note that changing any of the anteHandler or postHandler chain is
	// likely to be a state-machine breaking change, which needs a coordinated
	// upgrade.
	app.setPostHandler()

	// At startup, after all modules have been registered, check that all prot
	// annotations are correct.
	protoFiles, err := proto.MergedRegistry()
	if err != nil {
		panic(err)
	}
	err = msgservice.ValidateProtoAnnotations(protoFiles)
	if err != nil {
		// TODO: Once we switch to using protoreflect-based antehandlers, we might
		// want to panic here instead of logging a warning.
		fmt.Fprintln(os.Stderr, err.Error())
	}

	if loadLatest {
		if err := app.LoadLatestVersion(); err != nil {
			logger.Error("error on loading last version", "err", err)
			os.Exit(1)
		}
	}

	return app
}

func (app *EVMD) setAnteHandler(txConfig client.TxConfig, maxGasWanted uint64) {
	options := evmante.HandlerOptions{
		Cdc:                    app.appCodec,
		AccountKeeper:          app.AccountKeeper,
		BankKeeper:             app.BankKeeper,
		ExtensionOptionChecker: antetypes.HasDynamicFeeExtensionOption,
		EvmKeeper:              app.EVMKeeper,
		FeegrantKeeper:         app.FeeGrantKeeper,
		IBCKeeper:              app.IBCKeeper,
		FeeMarketKeeper:        app.FeeMarketKeeper,
		SignModeHandler:        txConfig.SignModeHandler(),
		SigGasConsumer:         evmante.SigVerificationGasConsumer,
		MaxTxGasWanted:         maxGasWanted,
		DynamicFeeChecker:      true,
		PendingTxListener:      app.onPendingTx,
	}
	if err := options.Validate(); err != nil {
		panic(err)
	}

	app.SetAnteHandler(evmante.NewAnteHandler(options))
}

func (app *EVMD) onPendingTx(hash common.Hash) {
	for _, listener := range app.pendingTxListeners {
		listener(hash)
	}
}

// RegisterPendingTxListener is used by json-rpc server to listen to pending transactions callback.
func (app *EVMD) RegisterPendingTxListener(listener func(common.Hash)) {
	app.pendingTxListeners = append(app.pendingTxListeners, listener)
}

func (app *EVMD) setPostHandler() {
	postHandler, err := posthandler.NewPostHandler(
		posthandler.HandlerOptions{},
	)
	if err != nil {
		panic(err)
	}

	app.SetPostHandler(postHandler)
}

// Name returns the name of the App
func (app *EVMD) Name() string { return app.BaseApp.Name() }

// BeginBlocker application updates every begin block
func (app *EVMD) BeginBlocker(ctx sdk.Context) (sdk.BeginBlock, error) {
	return app.ModuleManager.BeginBlock(ctx)
}

// EndBlocker application updates every end block
func (app *EVMD) EndBlocker(ctx sdk.Context) (sdk.EndBlock, error) {
	return app.ModuleManager.EndBlock(ctx)
}

func (app *EVMD) FinalizeBlock(req *abci.RequestFinalizeBlock) (res *abci.ResponseFinalizeBlock, err error) {
	return app.BaseApp.FinalizeBlock(req)
}

func (app *EVMD) Configurator() module.Configurator {
	return app.configurator
}

// InitChainer application update at chain initialization
func (app *EVMD) InitChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var genesisState GenesisState
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}

	if err := app.UpgradeKeeper.SetModuleVersionMap(ctx, app.ModuleManager.GetVersionMap()); err != nil {
		panic(err)
	}

	return app.ModuleManager.InitGenesis(ctx, app.appCodec, genesisState)
}

func (app *EVMD) PreBlocker(ctx sdk.Context, _ *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
	return app.ModuleManager.PreBlock(ctx)
}

// LoadHeight loads a particular height
func (app *EVMD) LoadHeight(height int64) error {
	return app.LoadVersion(height)
}

// LegacyAmino returns EVMD's amino codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *EVMD) LegacyAmino() *codec.LegacyAmino {
	return app.legacyAmino
}

// AppCodec returns EVMD's app codec.
//
// NOTE: This is solely to be used for testing purposes as it may be desirable
// for modules to register their own custom testing types.
func (app *EVMD) AppCodec() codec.Codec {
	return app.appCodec
}

// InterfaceRegistry returns EVMD's InterfaceRegistry
func (app *EVMD) InterfaceRegistry() types.InterfaceRegistry {
	return app.interfaceRegistry
}

// TxConfig returns EVMD's TxConfig
func (app *EVMD) TxConfig() client.TxConfig {
	return app.txConfig
}

// DefaultGenesis returns a default genesis from the registered AppModuleBasic's.
func (app *EVMD) DefaultGenesis() map[string]json.RawMessage {
	genesis := app.BasicModuleManager.DefaultGenesis(app.appCodec)

	mintGenState := NewMintGenesisState()
	genesis[minttypes.ModuleName] = app.appCodec.MustMarshalJSON(mintGenState)

	evmGenState := NewEVMGenesisState()
	genesis[evmtypes.ModuleName] = app.appCodec.MustMarshalJSON(evmGenState)

	// NOTE: for the example chain implementation we are also adding a default token pair,
	// which is the base denomination of the chain (i.e. the WEVMOS contract)
	erc20GenState := NewErc20GenesisState()
	genesis[erc20types.ModuleName] = app.appCodec.MustMarshalJSON(erc20GenState)

	return genesis
}

// GetKey returns the KVStoreKey for the provided store key.
//
// NOTE: This is solely to be used for testing purposes.
func (app *EVMD) GetKey(storeKey string) *storetypes.KVStoreKey {
	return app.keys[storeKey]
}

// SimulationManager implements the SimulationApp interface
func (app *EVMD) SimulationManager() *module.SimulationManager {
	return app.sm
}

// RegisterAPIRoutes registers all application module routes with the provided
// API server.
func (app *EVMD) RegisterAPIRoutes(apiSvr *api.Server, apiConfig config.APIConfig) {
	clientCtx := apiSvr.ClientCtx
	// Register new tx routes from grpc-gateway.
	authtx.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register new cometbft queries routes from grpc-gateway.
	cmtservice.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register node gRPC service for grpc-gateway.
	node.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// Register grpc-gateway routes for all modules.
	app.BasicModuleManager.RegisterGRPCGatewayRoutes(clientCtx, apiSvr.GRPCGatewayRouter)

	// register swagger API from root so that other applications can override easily
	if err := sdkserver.RegisterSwaggerAPI(apiSvr.ClientCtx, apiSvr.Router, apiConfig.Swagger); err != nil {
		panic(err)
	}
}

// RegisterTxService implements the Application.RegisterTxService method.
func (app *EVMD) RegisterTxService(clientCtx client.Context) {
	authtx.RegisterTxService(app.GRPCQueryRouter(), clientCtx, app.Simulate, app.interfaceRegistry)
}

// RegisterTendermintService implements the Application.RegisterTendermintService method.
func (app *EVMD) RegisterTendermintService(clientCtx client.Context) {
	cmtservice.RegisterTendermintService(
		clientCtx,
		app.GRPCQueryRouter(),
		app.interfaceRegistry,
		app.Query,
	)
}

func (app *EVMD) RegisterNodeService(clientCtx client.Context, cfg config.Config) {
	node.RegisterNodeService(clientCtx, app.GRPCQueryRouter(), cfg)
}

// ---------------------------------------------
// IBC Go TestingApp functions
//

// GetBaseApp implements the TestingApp interface.
func (app *EVMD) GetBaseApp() *baseapp.BaseApp {
	return app.BaseApp
}

// GetStakingKeeperSDK implements the TestingApp interface.
func (app *EVMD) GetStakingKeeperSDK() stakingkeeper.Keeper {
	return *app.StakingKeeper
}

// GetIBCKeeper implements the TestingApp interface.
func (app *EVMD) GetIBCKeeper() *ibckeeper.Keeper {
	return app.IBCKeeper
}

func (app *EVMD) GetEVMKeeper() *evmkeeper.Keeper {
	return app.EVMKeeper
}

func (app *EVMD) GetErc20Keeper() *erc20keeper.Keeper {
	return &app.Erc20Keeper
}

func (app *EVMD) SetErc20Keeper(erc20Keeper erc20keeper.Keeper) {
	app.Erc20Keeper = erc20Keeper
}

func (app *EVMD) GetGovKeeper() govkeeper.Keeper {
	return app.GovKeeper
}

func (app *EVMD) GetEvidenceKeeper() *evidencekeeper.Keeper {
	return &app.EvidenceKeeper
}

func (app *EVMD) GetSlashingKeeper() slashingkeeper.Keeper {
	return app.SlashingKeeper
}

func (app *EVMD) GetBankKeeper() bankkeeper.Keeper {
	return app.BankKeeper
}

func (app *EVMD) GetFeeMarketKeeper() *feemarketkeeper.Keeper {
	return &app.FeeMarketKeeper
}

func (app *EVMD) GetFeeGrantKeeper() feegrantkeeper.Keeper {
	return app.FeeGrantKeeper
}

func (app *EVMD) GetConsensusParamsKeeper() consensusparamkeeper.Keeper {
	return app.ConsensusParamsKeeper
}

func (app *EVMD) GetAccountKeeper() authkeeper.AccountKeeper {
	return app.AccountKeeper
}

func (app *EVMD) GetDistrKeeper() distrkeeper.Keeper {
	return app.DistrKeeper
}

func (app *EVMD) GetStakingKeeper() *stakingkeeper.Keeper {
	return app.StakingKeeper
}

func (app *EVMD) GetMintKeeper() mintkeeper.Keeper {
	return app.MintKeeper
}

func (app *EVMD) GetPreciseBankKeeper() *precisebankkeeper.Keeper {
	return &app.PreciseBankKeeper
}

func (app *EVMD) GetCallbackKeeper() ibccallbackskeeper.ContractKeeper {
	return app.CallbackKeeper
}

func (app *EVMD) GetTransferKeeper() transferkeeper.Keeper {
	return app.TransferKeeper
}

func (app *EVMD) SetTransferKeeper(transferKeeper transferkeeper.Keeper) {
	app.TransferKeeper = transferKeeper
}

func (app *EVMD) GetMempool() sdkmempool.ExtMempool {
	return app.EVMMempool
}

func (app *EVMD) GetAnteHandler() sdk.AnteHandler {
	return app.BaseApp.AnteHandler()
}

// GetTxConfig implements the TestingApp interface.
func (app *EVMD) GetTxConfig() client.TxConfig {
	return app.txConfig
}

func (app *EVMD) SetClientCtx(clientCtx client.Context) { // TODO:VLAD - Remove this if possible
	app.clientCtx = clientCtx
}

// Close unsubscribes from the CometBFT event bus (if set) and closes the mempool and underlying BaseApp.
func (app *EVMD) Close() error {
	var err error
	if m, ok := app.GetMempool().(*evmmempool.ExperimentalEVMMempool); ok && m != nil {
		app.Logger().Info("Shutting down mempool")
		err = m.Close()
	}

	msg := "Application gracefully shutdown"
	err = errors.Join(err, app.BaseApp.Close())
	if err == nil {
		app.Logger().Info(msg)
	} else {
		app.Logger().Error(msg, "error", err)
	}

	return err
}

// AutoCliOpts returns the autocli options for the app.
func (app *EVMD) AutoCliOpts() autocli.AppOptions {
	modules := make(map[string]appmodule.AppModule, 0)
	for _, m := range app.ModuleManager.Modules {
		if moduleWithName, ok := m.(module.HasName); ok {
			moduleName := moduleWithName.Name()
			if appModule, ok := moduleWithName.(appmodule.AppModule); ok {
				modules[moduleName] = appModule
			}
		}
	}

	return autocli.AppOptions{
		Modules:               modules,
		ModuleOptions:         runtimeservices.ExtractAutoCLIOptions(app.ModuleManager.Modules),
		AddressCodec:          evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
		ValidatorAddressCodec: evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32ValidatorAddrPrefix()),
		ConsensusAddressCodec: evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32ConsensusAddrPrefix()),
	}
}
