package precisebank

import (
	"encoding/json"
	"os"

	abci "github.com/cometbft/cometbft/abci/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm"
	eapp "github.com/cosmos/evm/evmd/app"
	"github.com/cosmos/evm/evmd/tests/integration"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebankmodule "github.com/cosmos/evm/x/precisebank"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
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

var _ evm.PreciseBankApp = (*PreciseBankPrecompileApp)(nil)

type PreciseBankPrecompileApp struct {
	eapp.App

	PreciseBankKeeper   precisebankkeeper.Keeper
	precisebankStoreKey *storetypes.KVStoreKey
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

	basicApp := eapp.New(
		logger,
		db,
		nil,
		loadLatest,
		appOptions,
		baseAppOptions...,
	)

	// wrap evm app with bank precompile app
	app := &PreciseBankPrecompileApp{
		App: *basicApp,
	}

	// add precisebank module permissioin to account keeper
	app.addModulePermissions()
	app.overrideModuleOrder()

	// add precisebank store key to app.storeKeys
	precisebankStoreKey := storetypes.NewKVStoreKey(precisebanktypes.StoreKey)
	app.extendEvmStoreKeys(precisebankStoreKey)

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

	// override init chainer to include precisebank genesis init
	app.SetInitChainer(app.initChainer)

	// load latest app state
	// This metthod seals the app, so it must be called after all keepers are set
	if err := app.LoadLatestVersion(); err != nil {
		panic(err)
	}

	return app
}

// Missing PreciseBankPrecompileApp interface methods

func (app *PreciseBankPrecompileApp) GetPreciseBankKeeper() *precisebankkeeper.Keeper {
	return &app.PreciseBankKeeper
}

func (app *PreciseBankPrecompileApp) GetKey(storeKey string) *storetypes.KVStoreKey {
	if storeKey == precisebanktypes.StoreKey {
		return app.precisebankStoreKey
	}

	return app.App.GetKey(storeKey)
}

// Helper funcitons
//
// Note: Dont't use this method in production code - only for test setup
// In production, store keys, abci method call orders, initChainer,
// and module permissions should be setup in app.go

// extendEvmStoreKeys records the precisebank store key inside the EVM keeper so its
// snapshot store (used during precompile execution) can see the precisebank KV store.
func (app *PreciseBankPrecompileApp) extendEvmStoreKeys(key storetypes.StoreKey) {
	evmStoreKeys := app.GetEVMKeeper().KVStoreKeys()
	if _, exists := evmStoreKeys[precisebanktypes.StoreKey]; exists {
		return
	}

	evmStoreKeys[precisebanktypes.StoreKey] = key
}

// overrideModuleOrder reproduces the base app's module ordering but inserts the
// precisebank module so it runs in begin/end blockers and genesis alongside the rest
// of the modules.
func (app *PreciseBankPrecompileApp) overrideModuleOrder() {
	app.ModuleManager.SetOrderBeginBlockers(
		minttypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		stakingtypes.ModuleName,
		genutiltypes.ModuleName,
		ibcexported.ModuleName,
		feemarkettypes.ModuleName,
		evmtypes.ModuleName,
		precisebanktypes.ModuleName,
	)

	app.ModuleManager.SetOrderEndBlockers(
		banktypes.ModuleName,
		govtypes.ModuleName,
		stakingtypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		authtypes.ModuleName,
		ibcexported.ModuleName,
		evmtypes.ModuleName,
		feemarkettypes.ModuleName,
		minttypes.ModuleName,
		genutiltypes.ModuleName,
		upgradetypes.ModuleName,
		precisebanktypes.ModuleName,
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
		feemarkettypes.ModuleName,
		genutiltypes.ModuleName,
		upgradetypes.ModuleName,
		consensustypes.ModuleName,
		precisebanktypes.ModuleName,
	}
	app.ModuleManager.SetOrderInitGenesis(initOrder...)
	app.ModuleManager.SetOrderExportGenesis(initOrder...)
}

// initChainer replays the default app.InitChainer and then manually invokes
// the precisebank module's InitGenesis. The main evmd application does not (yet)
// register the precisebank module with the module manager, so we have to call it here
// to ensure the keeper's state exists for tests that rely on precisebank module.
func (app *PreciseBankPrecompileApp) initChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var genesisState eapp.GenesisState
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}

	rawPrecisebankGenesis, ok := genesisState[precisebanktypes.ModuleName]
	if !ok {
		return app.App.InitChainer(ctx, req)
	}

	var precisebankGenesis precisebanktypes.GenesisState
	app.AppCodec().MustUnmarshalJSON(rawPrecisebankGenesis, &precisebankGenesis)

	resp, err := app.App.InitChainer(ctx, req)
	if err != nil {
		return resp, err
	}

	precisebankmodule.InitGenesis(ctx, app.PreciseBankKeeper, app.AccountKeeper, app.BankKeeper, &precisebankGenesis)

	return resp, nil
}

// addModulePermissions mirrors the production app's keeper wiring by
// registering the precisebank module account permissions after the fact.
func (app *PreciseBankPrecompileApp) addModulePermissions() {
	perms := app.AccountKeeper.GetModulePermissions()

	perms[precisebanktypes.ModuleName] = authtypes.NewPermissionsForAddress(
		precisebanktypes.ModuleName,
		[]string{authtypes.Minter, authtypes.Burner},
	)

	perms[erc20types.ModuleName] = authtypes.NewPermissionsForAddress(
		precisebanktypes.ModuleName,
		[]string{authtypes.Minter, authtypes.Burner},
	)
}
