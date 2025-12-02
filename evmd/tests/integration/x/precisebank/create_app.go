package precisebank

import (
	"encoding/json"
	"os"

	abci "github.com/cometbft/cometbft/abci/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm"
	eapp "github.com/cosmos/evm/evmd/app"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/evmd/testutil"
	srvflags "github.com/cosmos/evm/server/flags"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	precisebankmodule "github.com/cosmos/evm/x/precisebank"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
	return createEvmd(chainID, evmChainID, false, customBaseAppOptions...)
}

func CreateEvmdWithBlockSTM(chainID string, evmChainID uint64, customBaseAppOptions ...func(*baseapp.BaseApp)) evm.EvmApp {
	return createEvmd(chainID, evmChainID, true, customBaseAppOptions...)
}

func createEvmd(chainID string, evmChainID uint64, enableBlockSTM bool, customBaseAppOptions ...func(*baseapp.BaseApp)) evm.EvmApp {
	// A temporary home directory is created and used to prevent race conditions
	// related to home directory locks in chains that use the WASM module.
	defaultNodeHome, err := os.MkdirTemp("", "evmd-temp-homedir")
	if err != nil {
		panic(err)
	}

	// instantiate basic evm app
	db := dbm.NewMemDB()
	logger := log.NewNopLogger()
	loadLatest := false
	appOptions := integration.NewAppOptionsWithFlagHomeAndChainID(defaultNodeHome, evmChainID)
	if enableBlockSTM {
		appOptions[srvflags.EVMTxRunner] = "block-stm"
	}
	baseAppOptions := append(customBaseAppOptions, baseapp.SetChainID(chainID))
	evmApp := eapp.New(logger, db, nil, loadLatest, appOptions, baseAppOptions...)

	// wrap basic evmd app
	app := &PreciseBankPrecompileApp{
		App: *evmApp,
	}

	// add precisebank module permissioin to account keeper
	testutil.AddModulePermissions(app, precisebanktypes.ModuleName, true, true)
	testutil.AddModulePermissions(app, erc20types.ModuleName, true, true)

	// set precisebank keeper
	app.setPreciseBankKeeper()

	// override init chainer to include precisebank genesis init
	app.SetInitChainer(app.initChainer)

	// seal app
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

func (app *PreciseBankPrecompileApp) setPreciseBankKeeper() {
	// add precisebank store key to app.storeKeys
	precisebankStoreKey := storetypes.NewKVStoreKey(precisebanktypes.StoreKey)
	testutil.ExtendEvmStoreKey(app, precisebanktypes.ModuleName, precisebankStoreKey)

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
