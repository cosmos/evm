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
	"github.com/cosmos/evm/evmd/testutil"
	ics20precmopile "github.com/cosmos/evm/precompiles/ics20"
	srvflags "github.com/cosmos/evm/server/flags"
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
	ibccallbacks "github.com/cosmos/ibc-go/v10/modules/apps/callbacks"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcapi "github.com/cosmos/ibc-go/v10/modules/core/api"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/runtime"
	simutils "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var _ evm.IBCApp = (*IBCApp)(nil)

type IBCApp struct {
	eapp.App

	Erc20Keeper    erc20keeper.Keeper
	TransferKeeper transferkeeper.Keeper
	CallbackKeeper ibccallbackskeeper.ContractKeeper
}

// SetupEvmd initializes a new evmd app with default genesis state.
// It is used in IBC integration tests to create a new evmd app instance.
func SetupEvmd() (ibctesting.TestingApp, map[string]json.RawMessage) {
	defaultNodeHome, err := os.MkdirTemp("", "evmd-temp-homedir")
	if err != nil {
		panic(err)
	}

	// instantiate basic evm app
	logger := log.NewNopLogger()
	db := dbm.NewMemDB()
	loadLatest := false
	appOptions := NewAppOptionsWithFlagHomeAndChainID(defaultNodeHome, constants.EighteenDecimalsChainID)
	evmApp := eapp.New(logger, db, nil, loadLatest, appOptions)

	// wrap basic evmd app
	app := &IBCApp{
		App: *evmApp,
	}

	// add module permissions to account keeper
	testutil.AddModulePermissions(app, erc20types.ModuleName, true, true)
	testutil.AddModulePermissions(app, ibctransfertypes.ModuleName, true, true)

	// set keepers
	app.setERC20Keeper()
	app.setIBCTransferKeeper()
	app.setIBCTransferStack()

	// set ics20 precmopile
	app.setICS20Precompile()

	// override init chainer to include ERC20 and IBC transfer genesis execution
	app.SetInitChainer(app.initChainer)

	// set default genesis state
	genesisState := app.setDefaultGenesis()

	// seal app
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

func (app IBCApp) GetErc20Keeper() *erc20keeper.Keeper {
	return &app.Erc20Keeper
}

func (app IBCApp) GetTransferKeeper() transferkeeper.Keeper {
	return app.TransferKeeper
}

// Helper funcitons
//
// Note: Dont't use this method in production code - only for test setup
// In production, store keys, abci method call orders, initChainer,
// and module permissions should be setup in app.go

// set erc20 keeper to app
func (app *IBCApp) setERC20Keeper() {
	// mount erc20 store
	erc20StoreKey := storetypes.NewKVStoreKey(erc20types.StoreKey)
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
		&app.TransferKeeper,
	)
	app.GetEVMKeeper().SetErc20Keeper(&app.Erc20Keeper)

	// register erc20 interfaces so tx decoding works for x/erc20 tx msgs.
	erc20types.RegisterInterfaces(app.InterfaceRegistry())

	// register Msg service for ERC20 so MsgConvertERC20/ConvertCoin can be routed.
	erc20types.RegisterMsgServer(app.MsgServiceRouter(), &app.Erc20Keeper)
}

// set ibc transfer keeper to app
func (app *IBCApp) setIBCTransferKeeper() {
	// mount ibc transfer store
	ibcTransferStoreKey := storetypes.NewKVStoreKey(ibctransfertypes.StoreKey)
	app.MountStore(ibcTransferStoreKey, storetypes.StoreTypeIAVL)
	testutil.ExtendEvmStoreKey(app, ibctransfertypes.StoreKey, ibcTransferStoreKey)

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

// set ibc transfer stack to app
func (app *IBCApp) setIBCTransferStack() {
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

func (app *IBCApp) setICS20Precompile() {
	ics20Percompile := ics20precmopile.NewPrecompile(
		app.GetBankKeeper(),
		app.GetStakingKeeper(),
		app.GetTransferKeeper(),
		app.GetIBCKeeper().ChannelKeeper,
	)
	app.App.GetEVMKeeper().RegisterStaticPrecompile(ics20Percompile.Address(), ics20Percompile)
}

// initChainer replays the default app.InitChainer and then manually invokes
// the ERC20 & Transfer modules' InitGenesis. The main evmd application does not (yet)
// register the ERC20 & Transfer modules with the module manager, so we have to call them here
// to ensure the keeper's state exists for tests that rely on that modules.
func (app *IBCApp) initChainer(ctx sdk.Context, req *abcitypes.RequestInitChain) (*abcitypes.ResponseInitChain, error) {
	resp, err := app.App.InitChainer(ctx, req)
	if err != nil {
		return resp, err
	}

	var genesisState eapp.GenesisState
	if err := json.Unmarshal(req.AppStateBytes, &genesisState); err != nil {
		panic(err)
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

// setDefaultGenesis sets default genesis states of modules
func (app *IBCApp) setDefaultGenesis() map[string]json.RawMessage {
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
