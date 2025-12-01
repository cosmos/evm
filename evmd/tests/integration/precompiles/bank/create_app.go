package bank

import (
	"encoding/json"
	"os"

	abci "github.com/cometbft/cometbft/abci/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm"
	eapp "github.com/cosmos/evm/evmd/app"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/evmd/testutil"
	bankprecompile "github.com/cosmos/evm/precompiles/bank"
	erc20module "github.com/cosmos/evm/x/erc20"
	erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
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

var _ evm.BankPrecompileApp = (*BankPrecompileApp)(nil)

type BankPrecompileApp struct {
	eapp.App

	Erc20Keeper   erc20keeper.Keeper
	erc20StoreKey *storetypes.KVStoreKey
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
	app := &BankPrecompileApp{
		App: *eapp,
	}

	// add erc20 module permissioin to account keeper
	app.addErc20ModulePermissions()
	app.overrideModuleOrder()

	// add erc20 store key to app.storeKeys
	app.setERC20Keeper()

	// register bank precompile
	bankPrecmopile := bankprecompile.NewPrecompile(
		app.GetBankKeeper(),
		app.GetErc20Keeper(),
	)
	app.App.GetEVMKeeper().RegisterStaticPrecompile(bankPrecmopile.Address(), bankPrecmopile)

	// override init chainer to include erc20 genesis init
	app.SetInitChainer(app.bankInitChainer)

	// load latest app state
	// This metthod seals the app, so it must be called after all keepers are set
	if err := app.LoadLatestVersion(); err != nil {
		panic(err)
	}

	return app
}

// Missing BankPrecompileApp interface methods

func (app *BankPrecompileApp) GetErc20Keeper() *erc20keeper.Keeper {
	return &app.Erc20Keeper
}

func (app *BankPrecompileApp) GetKey(storeKey string) *storetypes.KVStoreKey {
	if storeKey == erc20types.StoreKey {
		return app.erc20StoreKey
	}

	return app.App.GetKey(storeKey)
}

// Helper funcitons
//
// Note: Dont't use this method in production code - only for test setup
// In production, store keys, abci method call orders, initChainer,
// and module permissions should be setup in app.go

func (app *BankPrecompileApp) setERC20Keeper() {
	// mount erc20 store
	erc20StoreKey := storetypes.NewKVStoreKey(erc20types.StoreKey)
	app.erc20StoreKey = erc20StoreKey
	app.MountStore(erc20StoreKey, storetypes.StoreTypeIAVL)
	testutil.ExtendEvmStoreKey(app, erc20types.StoreKey, erc20StoreKey)

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

	// register erc20 interfaces so tx decoding works for x/erc20 tx msgs.
	erc20types.RegisterInterfaces(app.InterfaceRegistry())

	// register Msg service for ERC20 so MsgConvertERC20/ConvertCoin can be routed.
	erc20types.RegisterMsgServer(app.MsgServiceRouter(), &app.Erc20Keeper)
}

// overrideModuleOrder reproduces the base app's module ordering but inserts the
// ERC20 module so it runs in begin/end blockers and genesis alongside the rest
// of the modules.
func (app *BankPrecompileApp) overrideModuleOrder() {
	app.ModuleManager.SetOrderBeginBlockers(
		minttypes.ModuleName,
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
func (app *BankPrecompileApp) bankInitChainer(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var genesisState eapp.GenesisState
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
	}

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

	return resp, nil
}

// addErc20ModulePermissions mirrors the production app's keeper wiring by
// registering the ERC20 module account permissions after the fact.
func (app *BankPrecompileApp) addErc20ModulePermissions() {
	perms := app.AccountKeeper.GetModulePermissions()
	if _, exists := perms[erc20types.ModuleName]; exists {
		return
	}

	perms[erc20types.ModuleName] = authtypes.NewPermissionsForAddress(
		erc20types.ModuleName,
		[]string{authtypes.Minter, authtypes.Burner},
	)
}
