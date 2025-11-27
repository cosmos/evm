package ante

import (
	"os"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm"
	eapp "github.com/cosmos/evm/evmd/app"
	"github.com/cosmos/evm/evmd/tests/integration"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/feegrant"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/runtime"
)

var _ evm.AnteIntegrationApp = (*AnteIntegrationApp)(nil)

type AnteIntegrationApp struct {
	eapp.App

	FeeGrantKeeper feegrantkeeper.Keeper
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
	app := &AnteIntegrationApp{
		App: *eapp,
	}

	// set keepers
	app.setFeeGrantKeeper()

	// load latest app state
	// This metthod seals the app, so it must be called after all keepers are set
	if err := app.LoadLatestVersion(); err != nil {
		panic(err)
	}

	return app
}

// Missing Erc20PrecompileApp interface methods
func (app AnteIntegrationApp) GetFeeGrantKeeper() feegrantkeeper.Keeper {
	return app.FeeGrantKeeper
}

// Helper funcitons
//
// Note: Dont't use this method in production code - only for test setup
// In production, store keys, abci method call orders, initChainer,
// and module permissions should be setup in app.go

// extendEvmStoreKey records the target store key inside the EVM keeper so its
// snapshot store (used during precompile execution) can see the target KV store.
func (app *AnteIntegrationApp) extendEvmStoreKey(keyName string, key storetypes.StoreKey) {
	evmStoreKeys := app.GetEVMKeeper().KVStoreKeys()
	evmStoreKeys[keyName] = key
}

func (app *AnteIntegrationApp) setFeeGrantKeeper() {
	// mount erc20 store
	feeGrantStoreKey := storetypes.NewKVStoreKey(feegrant.StoreKey)
	app.MountStore(feeGrantStoreKey, storetypes.StoreTypeIAVL)
	app.extendEvmStoreKey(feegrant.StoreKey, feeGrantStoreKey)

	// set erc20 keeper to app
	app.FeeGrantKeeper = feegrantkeeper.NewKeeper(
		app.AppCodec(),
		runtime.NewKVStoreService(feeGrantStoreKey),
		app.AccountKeeper,
	)

	// register erc20 interfaces so tx decoding works for x/erc20 tx msgs.
	feegrant.RegisterInterfaces(app.InterfaceRegistry())

	// register Msg service for ERC20 so MsgConvertERC20/ConvertCoin can be routed.
	feegrant.RegisterMsgServer(app.MsgServiceRouter(), feegrantkeeper.NewMsgServerImpl(app.FeeGrantKeeper))
}
