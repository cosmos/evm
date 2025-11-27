package erc20

import (
	"encoding/json"
	"os"

	abcitypes "github.com/cometbft/cometbft/abci/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm"
	evmaddress "github.com/cosmos/evm/encoding/address"
	eapp "github.com/cosmos/evm/evmd/app"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/testutil/constants"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/erc20"
	erc20module "github.com/cosmos/evm/x/erc20"
	erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	erc20v2 "github.com/cosmos/evm/x/erc20/v2"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	ibccallbackskeeper "github.com/cosmos/evm/x/ibc/callbacks/keeper"
	"github.com/cosmos/evm/x/ibc/transfer"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
	transferv2 "github.com/cosmos/evm/x/ibc/transfer/v2"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ibccallbacks "github.com/cosmos/ibc-go/v10/modules/apps/callbacks"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcapi "github.com/cosmos/ibc-go/v10/modules/core/api"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensustypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
)

var _ evm.Erc20PrecompileApp = (*Erc20PrecompileApp)(nil)

type Erc20PrecompileApp struct {
	eapp.App

	Erc20Keeper   erc20keeper.Keeper
	erc20StoreKey *storetypes.KVStoreKey

	PreciseBankKeeper   precisebankkeeper.Keeper
	precisebankStoreKey *storetypes.KVStoreKey

	TransferKeeper transferkeeper.Keeper
	transferKey    *storetypes.KVStoreKey

	CallbackKeeper ibccallbackskeeper.ContractKeeper
}

// CreateEvmd creates an evm app for regular integration tests (non-mempool)
// This version uses a noop mempool to avoid state issues during transaction processing
func CreateEvmd(chainID string, evmChainID uint64, customBaseAppOptions ...func(*baseapp.BaseApp)) evm.EvmApp {
	// A temporary home directory is created and used to prevent race conditions
	// related to home directory locks in chains that use the WASM module.
	defaultNodeHome, err := os.MkdirTemp("", "evmd-temp-homedir")
	if err != nil {
		panic(err)
	}

	db := dbm.NewMemDB()
	logger := log.NewNopLogger()
	loadLatest := false
	appOptions := integration.NewAppOptionsWithFlagHomeAndChainID(defaultNodeHome, evmChainID)
	baseAppOptions := append(customBaseAppOptions, baseapp.SetChainID(chainID))

	eapp := eapp.New(
		logger,
		db,
		nil,
		loadLatest,
		appOptions,
		baseAppOptions...,
	)

	// wrap evm app with bank precompile app
	app := &Erc20PrecompileApp{
		App: *eapp,
	}

	// add module permissioin to account keeper
	app.addModulePermissions()

	// set keepers
	app.setERC20Keeper()
	app.setPrecisebankKeeper()
	app.setIBCTransferKeeper()
	app.setIBCTransferStack()

	// override module order of abci interface calls
	app.overrideModuleOrder()

	// override init chainer to include erc20 genesis init
	app.SetInitChainer(app.initChainer)

	// load latest app state
	// This metthod seals the app, so it must be called after all keepers are set
	if err := app.LoadLatestVersion(); err != nil {
		panic(err)
	}

	return app
}

// Missing Erc20PrecompileApp interface methods

func (app Erc20PrecompileApp) GetErc20Keeper() *erc20keeper.Keeper {
	return &app.Erc20Keeper
}

func (app Erc20PrecompileApp) GetPreciseBankKeeper() *precisebankkeeper.Keeper {
	return &app.PreciseBankKeeper
}

func (app Erc20PrecompileApp) GetTransferKeeper() transferkeeper.Keeper {
	return app.TransferKeeper
}

// GetKey returns the KVStoreKey for the provided store key, including test-only modules.
func (app *Erc20PrecompileApp) GetKey(storeKey string) *storetypes.KVStoreKey {
	if storeKey == erc20types.StoreKey {
		return app.erc20StoreKey
	}
	if storeKey == ibctransfertypes.StoreKey {
		return app.transferKey
	}

	return app.App.GetKey(storeKey)
}

// Helper funcitons
//
// Note: Dont't use this method in production code - only for test setup
// In production, store keys, abci method call orders, initChainer,
// and module permissions should be setup in app.go

// extendEvmStoreKey records the target store key inside the EVM keeper so its
// snapshot store (used during precompile execution) can see the target KV store.
func (app *Erc20PrecompileApp) extendEvmStoreKey(keyName string, key storetypes.StoreKey) {
	evmStoreKeys := app.GetEVMKeeper().KVStoreKeys()
	evmStoreKeys[keyName] = key
}

func (app *Erc20PrecompileApp) setERC20Keeper() {
	// mount erc20 store
	erc20StoreKey := storetypes.NewKVStoreKey(erc20types.StoreKey)
	app.erc20StoreKey = erc20StoreKey
	app.MountStore(erc20StoreKey, storetypes.StoreTypeIAVL)
	app.extendEvmStoreKey(erc20types.StoreKey, erc20StoreKey)

	// set erc20 keeper to app
	app.Erc20Keeper = erc20keeper.NewKeeper(
		erc20StoreKey,
		app.AppCodec(),
		authtypes.NewModuleAddress(govtypes.ModuleName),
		app.AccountKeeper,
		app.BankKeeper,
		app.EVMKeeper,
		app.StakingKeeper,
		&app.TransferKeeper,
	)
	app.GetEVMKeeper().SetErc20Keeper(&app.Erc20Keeper)

	// register erc20 interfaces so tx decoding works for x/erc20 tx msgs.
	erc20types.RegisterInterfaces(app.InterfaceRegistry())

	// register Msg service for ERC20 so MsgConvertERC20/ConvertCoin can be routed.
	erc20types.RegisterMsgServer(app.MsgServiceRouter(), &app.Erc20Keeper)
}

func (app *Erc20PrecompileApp) setPrecisebankKeeper() {
	// add precisebank store key to app.storeKeys
	precisebankStoreKey := storetypes.NewKVStoreKey(precisebanktypes.StoreKey)
	app.extendEvmStoreKey(precisebanktypes.StoreKey, precisebankStoreKey)

	// mount precisebank store
	app.MountStore(precisebankStoreKey, storetypes.StoreTypeIAVL)
	app.precisebankStoreKey = precisebankStoreKey

	// set precisebank keeper to app
	app.PreciseBankKeeper = precisebankkeeper.NewKeeper(
		app.AppCodec(),
		precisebankStoreKey,
		app.BankKeeper,
		app.AccountKeeper,
	)
	app.EVMKeeper.SetBankKeeper(app.PreciseBankKeeper)
}

func (app *Erc20PrecompileApp) setIBCTransferKeeper() {
	// mount ibc transfer store
	ibcTransferStoreKey := storetypes.NewKVStoreKey(ibctransfertypes.StoreKey)
	app.transferKey = ibcTransferStoreKey
	app.MountStore(ibcTransferStoreKey, storetypes.StoreTypeIAVL)
	app.extendEvmStoreKey(ibctransfertypes.StoreKey, ibcTransferStoreKey)

	// get authority address
	authAddr := authtypes.NewModuleAddress(govtypes.ModuleName).String()

	// set ibc transfer keeper to app
	app.TransferKeeper = transferkeeper.NewKeeper(
		app.AppCodec(),
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
		runtime.NewKVStoreService(ibcTransferStoreKey),
		app.IBCKeeper.ChannelKeeper,
		app.MsgServiceRouter(),
		app.AccountKeeper,
		app.BankKeeper,
		app.Erc20Keeper, // Add ERC20 Keeper for ERC20 transfers
		authAddr,
	)

	// register IBC transfer interfaces so tx decoding works for ICS20 msgs.
	ibctransfertypes.RegisterInterfaces(app.InterfaceRegistry())

	// register Msg service for ICS20 so MsgTransfer can be routed.
	ibctransfertypes.RegisterMsgServer(app.MsgServiceRouter(), app.TransferKeeper)
}

func (app *Erc20PrecompileApp) setIBCTransferStack() {
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
}

// overrideModuleOrder reproduces the base app's module ordering but inserts the
// ERC20 module so it runs in begin/end blockers and genesis alongside the rest
// of the modules.
func (app *Erc20PrecompileApp) overrideModuleOrder() {
	app.ModuleManager.SetOrderBeginBlockers(
		minttypes.ModuleName,
		ibcexported.ModuleName,
		ibctransfertypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		stakingtypes.ModuleName,
		genutiltypes.ModuleName,
		ibcexported.ModuleName,
		feemarkettypes.ModuleName,
		erc20types.ModuleName,
		evmtypes.ModuleName,
	)

	app.ModuleManager.SetOrderEndBlockers(
		banktypes.ModuleName,
		govtypes.ModuleName,
		stakingtypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		authtypes.ModuleName,
		ibcexported.ModuleName,
		erc20types.ModuleName,
		evmtypes.ModuleName,
		feemarkettypes.ModuleName,
		minttypes.ModuleName,
		genutiltypes.ModuleName,
		upgradetypes.ModuleName,
	)

	initOrder := []string{
		authtypes.ModuleName,
		banktypes.ModuleName,
		distrtypes.ModuleName,
		stakingtypes.ModuleName,
		slashingtypes.ModuleName,
		govtypes.ModuleName,
		minttypes.ModuleName,
		ibcexported.ModuleName,
		evmtypes.ModuleName,
		erc20types.ModuleName,
		feemarkettypes.ModuleName,
		ibctransfertypes.ModuleName,
		genutiltypes.ModuleName,
		upgradetypes.ModuleName,
		consensustypes.ModuleName,
	}
	app.ModuleManager.SetOrderInitGenesis(initOrder...)
	app.ModuleManager.SetOrderExportGenesis(initOrder...)
}

// initChainer replays the default app.InitChainer and then manually invokes
// the ERC20 module's InitGenesis. The main evmd application does not (yet)
// register the ERC20 module with the module manager, so we have to call it here
// to ensure the keeper's state exists for tests that rely on ERC20 module.
func (app *Erc20PrecompileApp) initChainer(ctx sdk.Context, req *abcitypes.RequestInitChain) (*abcitypes.ResponseInitChain, error) {
	var genesisState eapp.GenesisState
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}

	resp, err := app.App.InitChainer(ctx, req)
	if err != nil {
		return resp, err
	}

	// ensure module accounts exist for test-only modules at runtime
	if app.AccountKeeper.GetModuleAccount(ctx, erc20types.ModuleName) == nil {
		app.AccountKeeper.SetModuleAccount(ctx, authtypes.NewEmptyModuleAccount(erc20types.ModuleName, authtypes.Minter, authtypes.Burner))
	}
	if app.AccountKeeper.GetModuleAccount(ctx, ibctransfertypes.ModuleName) == nil {
		app.AccountKeeper.SetModuleAccount(ctx, authtypes.NewEmptyModuleAccount(ibctransfertypes.ModuleName, authtypes.Minter, authtypes.Burner))
	}

	// erc20 module init genesis (if provided)
	if rawErc20Genesis, ok := genesisState[erc20types.ModuleName]; ok {
		var erc20Genesis erc20types.GenesisState
		app.AppCodec().MustUnmarshalJSON(rawErc20Genesis, &erc20Genesis)
		erc20module.InitGenesis(ctx, app.Erc20Keeper, app.AccountKeeper, erc20Genesis)
	}

	// ibc transfer module init genesis (if provided)
	if rawTransferGenesis, ok := genesisState[ibctransfertypes.ModuleName]; ok {
		transferModule := transfer.NewAppModule(app.TransferKeeper)
		transferModule.InitGenesis(ctx, app.AppCodec(), rawTransferGenesis)
	}

	return resp, nil
}

// setdefaultGenesis sets default genesis states of modules
func (app *Erc20PrecompileApp) setDefaultGenesis() map[string]json.RawMessage {
	// disable base fee for testing
	genesisState := app.DefaultGenesis()
	fmGen := feemarkettypes.DefaultGenesisState()
	fmGen.Params.NoBaseFee = true
	genesisState[feemarkettypes.ModuleName] = app.AppCodec().MustMarshalJSON(fmGen)
	stakingGen := stakingtypes.DefaultGenesisState()
	stakingGen.Params.BondDenom = constants.ExampleAttoDenom
	genesisState[stakingtypes.ModuleName] = app.AppCodec().MustMarshalJSON(stakingGen)
	mintGen := minttypes.DefaultGenesisState()
	mintGen.Params.MintDenom = constants.ExampleAttoDenom
	genesisState[minttypes.ModuleName] = app.AppCodec().MustMarshalJSON(mintGen)

	// set default erc20 module genesis state
	erc20GenState := erc20types.DefaultGenesisState()
	erc20GenState.TokenPairs = testconstants.ExampleTokenPairs
	erc20GenState.NativePrecompiles = []string{testconstants.WEVMOSContractMainnet}
	genesisState[erc20types.ModuleName] = app.AppCodec().MustMarshalJSON(erc20GenState)

	// ensure transfer module has a genesis entry so InitGenesis sets the port
	// and avoids "invalid port: transfer" errors during channel creation
	transferGen := ibctransfertypes.DefaultGenesisState()
	genesisState[ibctransfertypes.ModuleName] = app.AppCodec().MustMarshalJSON(transferGen)

	return genesisState
}

// addModulePermissions mirrors the production app's keeper wiring by
// registering the module account permissions after the fact.
func (app *Erc20PrecompileApp) addModulePermissions() {
	perms := app.AccountKeeper.GetModulePermissions()
	if _, exists := perms[erc20types.ModuleName]; exists {
		return
	}

	perms[erc20types.ModuleName] = authtypes.NewPermissionsForAddress(
		erc20types.ModuleName,
		[]string{authtypes.Minter, authtypes.Burner},
	)

	perms[ibctransfertypes.ModuleName] = authtypes.NewPermissionsForAddress(
		ibctransfertypes.ModuleName,
		[]string{authtypes.Minter, authtypes.Burner},
	)
}
