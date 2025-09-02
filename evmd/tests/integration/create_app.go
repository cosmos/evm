package integration

import (
	"encoding/json"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd"
	testconfig "github.com/cosmos/evm/testutil/config"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	clienthelpers "cosmossdk.io/client/v2/helpers"
	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/baseapp"
	simutils "github.com/cosmos/cosmos-sdk/testutil/sims"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// testEvmAppOptionsWithReset creates an EvmAppOptions function for testing with reset
func testEvmAppOptionsWithReset(evmChainID uint64) error {
	// Use default chain configuration for testing
	chainConfig := testconfig.DefaultChainConfig
	configurator := evmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	return configurator.WithEVMCoinInfo(chainConfig.CoinInfo).Configure()
}

// CreateEvmd creates an evm app for regular integration tests (non-mempool)
// This version uses a noop mempool to avoid state issues during transaction processing
func CreateEvmd(chainID string, evmChainID uint64, customBaseAppOptions ...func(*baseapp.BaseApp)) evm.EvmApp {
	defaultNodeHome, err := clienthelpers.GetNodeHomeDirectory(".evmd")
	if err != nil {
		panic(err)
	}

	db := dbm.NewMemDB()
	logger := log.NewNopLogger()
	loadLatest := true
	appOptions := simutils.NewAppOptionsWithFlagHome(defaultNodeHome)

	baseAppOptions := append(customBaseAppOptions, baseapp.SetChainID(chainID))

	return evmd.NewExampleApp(
		logger,
		db,
		nil,
		loadLatest,
		appOptions,
		evmChainID,
		testEvmAppOptionsWithReset,
		baseAppOptions...,
	)
}

// SetupEvmd initializes a new evmd app with default genesis state.
// It is used in IBC integration tests to create a new evmd app instance.
func SetupEvmd() (ibctesting.TestingApp, map[string]json.RawMessage) {
	chainID := testconfig.DefaultChainConfig.ChainInfo.EVMChainID
	app := evmd.NewExampleApp(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		simutils.EmptyAppOptions{},
		chainID,
		testEvmAppOptionsWithReset,
	)
	// disable base fee for testing
	genesisState := app.DefaultGenesis()
	fmGen := feemarkettypes.DefaultGenesisState()
	fmGen.Params.NoBaseFee = true
	genesisState[feemarkettypes.ModuleName] = app.AppCodec().MustMarshalJSON(fmGen)
	stakingGen := stakingtypes.DefaultGenesisState()
	stakingGen.Params.BondDenom = testconfig.DefaultChainConfig.CoinInfo.Denom
	genesisState[stakingtypes.ModuleName] = app.AppCodec().MustMarshalJSON(stakingGen)
	mintGen := minttypes.DefaultGenesisState()
	mintGen.Params.MintDenom = testconfig.DefaultChainConfig.CoinInfo.Denom
	genesisState[minttypes.ModuleName] = app.AppCodec().MustMarshalJSON(mintGen)

	return app, genesisState
}
