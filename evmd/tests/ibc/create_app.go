package ibc

import (
	"encoding/json"
	"os"

	abcitypes "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/cosmos-sdk/client/flags"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm"
	evmaddress "github.com/cosmos/evm/encoding/address"
	eapp "github.com/cosmos/evm/evmd/app"
	srvflags "github.com/cosmos/evm/server/flags"
	"github.com/cosmos/evm/testutil/constants"
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
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ibccallbacks "github.com/cosmos/ibc-go/v10/modules/apps/callbacks"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcapi "github.com/cosmos/ibc-go/v10/modules/core/api"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/runtime"
	simutils "github.com/cosmos/cosmos-sdk/testutil/sims"
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

var _ evm.IBCApp = (*IBCApp)(nil)

type IBCApp struct {
	eapp.App

	Erc20Keeper   erc20keeper.Keeper
	erc20StoreKey *storetypes.KVStoreKey

	TransferKeeper transferkeeper.Keeper
	transferKey    *storetypes.KVStoreKey

	CallbackKeeper ibccallbackskeeper.ContractKeeper
}

// SetupEvmd initializes a new evmd app with default genesis state.
// It is used in IBC integration tests to create a new evmd app instance.
func SetupEvmd() (ibctesting.TestingApp, map[string]json.RawMessage) {
	defaultNodeHome, err := os.MkdirTemp("", "evmd-temp-homedir")
	if err != nil {
		panic(err)
	}

	logger := log.NewNopLogger()
	db := dbm.NewMemDB()
	loadLatest := false
	appOptions := NewAppOptionsWithFlagHomeAndChainID(defaultNodeHome, constants.EighteenDecimalsChainID)

	eapp := eapp.New(
		logger,
		db,
		nil,
		loadLatest,
		appOptions,
	)

	// wrap evm app with bank precompile app
	app := &IBCApp{
		App: *eapp,
	}

	// add erc20 module permissioin to account keeper
	app.addErc20ModulePermissions()
	app.overrideModuleOrder()

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
		nil,
	)
	app.GetEVMKeeper().SetErc20Keeper(&app.Erc20Keeper)

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

	// load latest app state
	// This metthod seals the app, so it must be called after all keepers are set
	if err := app.LoadLatestVersion(); err != nil {
		panic(err)
	}

	return app, genesisState
}

func NewAppOptionsWithFlagHomeAndChainID(home string, evmChainID uint64) simutils.AppOptionsMap {
	return simutils.AppOptionsMap{
		flags.FlagHome:      home,
		srvflags.EVMChainID: evmChainID,
	}
}

// Helper funcitons
//
// Note: Dont't use this method in production code - only for test setup
// In production, store keys, abci method call orders, initChainer,
// and module permissions should be setup in app.go

// extendEvmStoreKeys records the target store key inside the EVM keeper so its
// snapshot store (used during precompile execution) can see the target KV store.
func (app *IBCApp) extendEvmStoreKey(keyName string, key storetypes.StoreKey) {
	evmStoreKeys := app.GetEVMKeeper().KVStoreKeys()
	evmStoreKeys[keyName] = key
}

// GetKey returns the KVStoreKey for the provided store key, including test-only modules.
func (app *IBCApp) GetKey(storeKey string) *storetypes.KVStoreKey {
	if storeKey == erc20types.StoreKey {
		return app.erc20StoreKey
	}
	if storeKey == ibctransfertypes.StoreKey {
		return app.transferKey
	}

	return app.App.GetKey(storeKey)
}

// overrideModuleOrder reproduces the base app's module ordering but inserts the
// ERC20 module so it runs in begin/end blockers and genesis alongside the rest
// of the modules.
func (app *IBCApp) overrideModuleOrder() {
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

// bankInitChainer replays the default app.InitChainer and then manually invokes
// the ERC20 module's InitGenesis. The main evmd application does not (yet)
// register the ERC20 module with the module manager, so we have to call it here
// to ensure the keeper's state exists for tests that rely on ERC20 module.
func (app *IBCApp) bankInitChainer(ctx sdk.Context, req *abcitypes.RequestInitChain) (*abcitypes.ResponseInitChain, error) {
	var genesisState eapp.GenesisState
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}

	// erc20 module init genesis
	rawErc20Genesis, ok := genesisState[erc20types.ModuleName]
	if !ok {
		return app.App.InitChainer(ctx, req)
	}

	var erc20Genesis erc20types.GenesisState
	app.AppCodec().MustUnmarshalJSON(rawErc20Genesis, &erc20Genesis)

	resp, err := app.App.InitChainer(ctx, req)
	if err != nil {
		return resp, err
	}

	erc20module.InitGenesis(ctx, app.Erc20Keeper, app.AccountKeeper, erc20Genesis)

	// ibc transfer module init genesis
	rawTransferGenesis, ok := genesisState[ibctransfertypes.ModuleName]
	if !ok {
		return app.App.InitChainer(ctx, req)
	}

	transferModule := transfer.NewAppModule(app.TransferKeeper)
	transferModule.InitGenesis(ctx, app.AppCodec(), rawTransferGenesis)

	return resp, nil
}

// addErc20ModulePermissions mirrors the production app's keeper wiring by
// registering the ERC20 module account permissions after the fact.
func (app *IBCApp) addErc20ModulePermissions() {
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
