package distribution

import (
	"os"

	"github.com/cosmos/evm"
	evmaddress "github.com/cosmos/evm/encoding/address"
	eapp "github.com/cosmos/evm/evmd/app"
	"github.com/cosmos/evm/evmd/tests/integration"
	distrprecompile "github.com/cosmos/evm/precompiles/distribution"

	dbm "github.com/cosmos/cosmos-db"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
)

var _ evm.DistributionPrecompileApp = (*DistributionPrecompileApp)(nil)

type DistributionPrecompileApp struct {
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

	db := dbm.NewMemDB()
	logger := log.NewNopLogger()
	loadLatest := true
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

	app := &DistributionPrecompileApp{
		App: *eapp,
	}

	distributionPrecompile := distrprecompile.NewPrecompile(
		app.GetDistrKeeper(),
		distrkeeper.NewMsgServerImpl(app.GetDistrKeeper()),
		distrkeeper.NewQuerier(app.GetDistrKeeper()),
		app.GetStakingKeeper(),
		app.GetBankKeeper(),
		evmaddress.NewEvmCodec(sdktypes.GetConfig().GetBech32AccountAddrPrefix()),
	)

	app.App.GetEVMKeeper().RegisterStaticPrecompile(distributionPrecompile.Address(), distributionPrecompile)

	return app
}

func (app *DistributionPrecompileApp) GetSlashingKeeper() slashingkeeper.Keeper {
	return app.SlashingKeeper
}

func (app *DistributionPrecompileApp) GetDistrKeeper() distrkeeper.Keeper {
	return app.DistributionKeeper
}
