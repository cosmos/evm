package integration

import (
	"encoding/json"
	"os"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd"
	srvflags "github.com/cosmos/evm/server/flags"
	"github.com/cosmos/evm/testutil/constants"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	ibctesting "github.com/cosmos/ibc-go/v11/testing"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	simutils "github.com/cosmos/cosmos-sdk/testutil/sims"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// CreateEvmd creates an evm app for integration tests
func CreateEvmd(chainID string, evmChainID uint64, exclusiveMempool bool, customBaseAppOptions ...func(*baseapp.BaseApp)) evm.EvmApp {
	// A temporary home directory is created and used to prevent race conditions
	// related to home directory locks in chains that use the WASM module.
	defaultNodeHome, err := os.MkdirTemp("", "evmd-temp-homedir")
	if err != nil {
		panic(err)
	}

	db := dbm.NewMemDB()
	logger := log.NewNopLogger()
	loadLatest := true
	appOptions := NewAppOptionsWithFlagHomeAndChainID(defaultNodeHome, evmChainID, exclusiveMempool)

	baseAppOptions := append(customBaseAppOptions, baseapp.SetChainID(chainID))

	// Start the app
	app := evmd.NewExampleApp(
		logger,
		db,
		nil,
		loadLatest,
		appOptions,
		baseAppOptions...,
	)

	// Prepare the client context
	clientCtx := client.Context{}.WithChainID(constants.ExampleChainID.ChainID).
		WithHeight(1).
		WithTxConfig(app.GetTxConfig())

	// Get the mempool and set the client context if supported
	if m, ok := app.GetMempool().(interface{ SetClientCtx(client.Context) }); ok && m != nil {
		m.SetClientCtx(clientCtx)
	}

	return app
}

// SetupEvmd initializes a new evmd app with default genesis state.
// It is used in IBC integration tests to create a new evmd app instance.
func SetupEvmd() (ibctesting.TestingApp, map[string]json.RawMessage) {
	defaultNodeHome, err := os.MkdirTemp("", "evmd-temp-homedir")
	if err != nil {
		panic(err)
	}

	app := evmd.NewExampleApp(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		NewAppOptionsWithFlagHomeAndChainID(defaultNodeHome, constants.EighteenDecimalsChainID, false),
	)
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

	return app, genesisState
}

func NewAppOptionsWithFlagHomeAndChainID(home string, evmChainID uint64, exlcusiveMempool bool) simutils.AppOptionsMap {
	return simutils.AppOptionsMap{
		flags.FlagHome:                              home,
		srvflags.EVMChainID:                         evmChainID,
		srvflags.EVMMempoolOperateExclusively:       exlcusiveMempool,
		srvflags.EVMMempoolInsertQueueSize:          5000,
		srvflags.EVMMempoolPendingTxProposalTimeout: "250ms",
	}
}
