package slashing

import (
	"os"

	"github.com/cosmos/evm"
	evmaddress "github.com/cosmos/evm/encoding/address"
	eapp "github.com/cosmos/evm/evmd/app"
	"github.com/cosmos/evm/evmd/tests/integration"
	slashingprecompile "github.com/cosmos/evm/precompiles/slashing"

	dbm "github.com/cosmos/cosmos-db"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
)

var _ evm.SlashingPrecompileApp = (*SlashingPrecompileApp)(nil)

type SlashingPrecompileApp struct {
	eapp.App
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

	// instantiate basic evm app
	db := dbm.NewMemDB()
	logger := log.NewNopLogger()
	loadLatest := true
	appOptions := integration.NewAppOptionsWithFlagHomeAndChainID(defaultNodeHome, evmChainID)
	baseAppOptions := append(customBaseAppOptions, baseapp.SetChainID(chainID))
	evmApp := eapp.New(logger, db, nil, loadLatest, appOptions, baseAppOptions...)

	// wrap basic evmd app
	app := &SlashingPrecompileApp{
		App: *evmApp,
	}

	// set slashing precompile
	app.setSlashingPrecompile()

	return app
}

func (app *SlashingPrecompileApp) GetSlashingKeeper() slashingkeeper.Keeper {
	return app.SlashingKeeper
}

func (app *SlashingPrecompileApp) GetDistrKeeper() distrkeeper.Keeper {
	return app.DistributionKeeper
}

func (app *SlashingPrecompileApp) setSlashingPrecompile() {
	slashingPrecompile := slashingprecompile.NewPrecompile(
		app.GetSlashingKeeper(),
		slashingkeeper.NewMsgServerImpl(app.GetSlashingKeeper()),
		app.GetBankKeeper(),
		evmaddress.NewEvmCodec(sdktypes.GetConfig().GetBech32ValidatorAddrPrefix()),
		evmaddress.NewEvmCodec(sdktypes.GetConfig().GetBech32ConsensusAddrPrefix()),
	)
	app.App.GetEVMKeeper().RegisterStaticPrecompile(slashingPrecompile.Address(), slashingPrecompile)
}
