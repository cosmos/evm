package testutil

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"

	dbm "github.com/cosmos/cosmos-db"
	evmconfig "github.com/cosmos/evm/config"
	evmdapp "github.com/cosmos/evm/evmd/app"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	"cosmossdk.io/log"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// SetupOptions defines arguments that are passed into `Simapp` constructor.
type SetupOptions struct {
	Logger  log.Logger
	DB      *dbm.MemDB
	AppOpts servertypes.AppOptions
}

func init() {
	// we're setting the minimum gas price to 0 to simplify the tests
	feemarkettypes.DefaultMinGasPrice = math.LegacyZeroDec()

	// Set the global SDK config for the tests
	cfg := sdk.GetConfig()
	evmconfig.SetBech32Prefixes(cfg)
	evmconfig.SetBip44CoinType(cfg)
}

func setup(withGenesis bool, invCheckPeriod uint, chainID string, evmChainID uint64) (*evmdapp.EVMD, evmdapp.GenesisState) {
	db := dbm.NewMemDB()

	appOptions := make(simtestutil.AppOptionsMap, 0)
	appOptions[flags.FlagHome] = evmdapp.DefaultNodeHome
	appOptions[server.FlagInvCheckPeriod] = invCheckPeriod

	chainConfig := evmconfig.DefaultChainConfig
	app := evmdapp.NewExampleApp(log.NewNopLogger(), db, nil, true, appOptions, baseapp.SetChainID(chainID))
	if withGenesis {
		return app, app.DefaultGenesis()
	}

	return app, evmdapp.GenesisState{}
}

// Setup initializes a new EVMD. A Nop logger is set in EVMD.
func Setup(t *testing.T, chainID string, evmChainID uint64) *evmdapp.EVMD {
	t.Helper()

	privVal := mock.NewPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(t, err)

	// create validator set with single validator
	validator := cmttypes.NewValidator(pubKey, 1)
	valSet := cmttypes.NewValidatorSet([]*cmttypes.Validator{validator})

	// generate genesis account
	senderPrivKey := secp256k1.GenPrivKey()
	acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(100000000000000))),
	}

	app := SetupWithGenesisValSet(t, chainID, evmChainID, valSet, []authtypes.GenesisAccount{acc}, balance)

	return app
}

// SetupWithGenesisValSet initializes a new EVMD with a validator set and genesis accounts
// that also act as delegators. For simplicity, each validator is bonded with a delegation
// of one consensus engine unit in the default token of the simapp from first genesis
// account. A Nop logger is set in EVMD.
func SetupWithGenesisValSet(t *testing.T, chainID string, evmChainID uint64, valSet *cmttypes.ValidatorSet, genAccs []authtypes.GenesisAccount, balances ...banktypes.Balance) *evmdapp.EVMD {
	t.Helper()

	app, genesisState := setup(true, 5, chainID, evmChainID)
	genesisState, err := simtestutil.GenesisStateWithValSet(app.AppCodec(), genesisState, valSet, genAccs, balances...)
	require.NoError(t, err)

	stateBytes, err := json.MarshalIndent(genesisState, "", " ")
	require.NoError(t, err)

	// init chain will set the validator set and initialize the genesis accounts
	if _, err = app.InitChain(
		&abci.RequestInitChain{
			Validators:      []abci.ValidatorUpdate{},
			ConsensusParams: simtestutil.DefaultConsensusParams,
			AppStateBytes:   stateBytes,
			ChainId:         chainID,
		},
	); err != nil {
		panic(fmt.Sprintf("app.InitChain failed: %v", err))
	}

	// NOTE: we are NOT committing the changes here as opposed to the function from simapp
	// because that would already adjust e.g. the base fee in the params.
	// We want to keep the genesis state as is for the tests unless we commit the changes manually.

	return app
}

// SetupTestingApp initializes the IBC-go testing application
// need to keep this design to comply with the ibctesting SetupTestingApp func
// and be able to set the chainID for the tests properly
func SetupTestingApp(chainConfig evmconfig.ChainConfig) func() (ibctesting.TestingApp, map[string]json.RawMessage) {
	return func() (ibctesting.TestingApp, map[string]json.RawMessage) {
		db := dbm.NewMemDB()
		app := evmdapp.NewExampleApp(log.NewNopLogger(), db, nil, true, simtestutil.NewAppOptionsWithFlagHome(evmdapp.DefaultNodeHome), baseapp.SetChainID(chainConfig.ChainID))
		return app, app.DefaultGenesis()
	}
}
