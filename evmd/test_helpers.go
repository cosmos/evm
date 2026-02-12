package evmd

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cosmos/evm/evmd/config"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/x/vm/types"

	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"

	dbm "github.com/cosmos/cosmos-db"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	"cosmossdk.io/log"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	poatypes "github.com/cosmos/cosmos-sdk/enterprise/poa/x/poa/types"
	"github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
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
	config.SetBech32Prefixes(cfg)
	config.SetBip44CoinType(cfg)
}

func setup(withGenesis bool, invCheckPeriod uint, chainID string, evmChainID uint64) (*EVMD, GenesisState) {
	db := dbm.NewMemDB()

	appOptions := make(simtestutil.AppOptionsMap, 0)
	appOptions[flags.FlagHome] = defaultNodeHome
	appOptions[server.FlagInvCheckPeriod] = invCheckPeriod

	app := NewExampleApp(log.NewNopLogger(), db, nil, true, appOptions, baseapp.SetChainID(chainID))
	if withGenesis {
		return app, app.DefaultGenesis()
	}

	return app, GenesisState{}
}

// Setup initializes a new EVMD. A Nop logger is set in EVMD.
func Setup(t *testing.T, chainID string, evmChainID uint64) *EVMD {
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
		Coins:   sdk.NewCoins(sdk.NewCoin(types.DefaultEVMExtendedDenom, math.NewInt(100000000000000))),
	}

	app := SetupWithGenesisValSet(t, chainID, evmChainID, valSet, []authtypes.GenesisAccount{acc}, balance)

	return app
}

// SetupWithGenesisValSet initializes a new EVMD with a validator set and genesis accounts.
// In POA mode, validators are added directly to the POA genesis state instead of
// using staking delegations. A Nop logger is set in EVMD.
func SetupWithGenesisValSet(t *testing.T, chainID string, evmChainID uint64, valSet *cmttypes.ValidatorSet, genAccs []authtypes.GenesisAccount, balances ...banktypes.Balance) *EVMD {
	t.Helper()

	app, genesisState := setup(true, 5, chainID, evmChainID)

	// Set auth genesis with provided accounts
	authGenesis := authtypes.NewGenesisState(authtypes.DefaultParams(), genAccs)
	genesisState[authtypes.ModuleName] = app.AppCodec().MustMarshalJSON(authGenesis)

	// Create POA validators from the CometBFT validator set
	poaValidators := make([]poatypes.Validator, 0, len(valSet.Validators))
	for _, val := range valSet.Validators {
		pk, err := cryptocodec.FromCmtPubKeyInterface(val.PubKey)
		require.NoError(t, err)

		pkAny, err := codectypes.NewAnyWithValue(pk)
		require.NoError(t, err)

		poaValidators = append(poaValidators, poatypes.Validator{
			PubKey: pkAny,
			Power:  val.VotingPower,
		})
	}

	// Set POA genesis with validators
	poaGenState := poatypes.GenesisState{
		Params:     poatypes.Params{Admin: authtypes.NewModuleAddress(govtypes.ModuleName).String()},
		Validators: poaValidators,
	}
	genesisState[poatypes.ModuleName] = app.AppCodec().MustMarshalJSON(&poaGenState)

	// Calculate total supply from balances
	totalSupply := sdk.NewCoins()
	for _, b := range balances {
		totalSupply = totalSupply.Add(b.Coins...)
	}

	// Set bank genesis with balances, total supply, and denom metadata
	bankGenesis := banktypes.NewGenesisState(
		banktypes.DefaultGenesisState().Params,
		balances,
		totalSupply,
		network.GenerateBankGenesisMetadata(evmChainID),
		[]banktypes.SendEnabled{},
	)
	genesisState[banktypes.ModuleName] = app.AppCodec().MustMarshalJSON(bankGenesis)

	// Set EVM genesis with the correct denom
	var evmGenesis types.GenesisState
	app.AppCodec().MustUnmarshalJSON(genesisState[types.ModuleName], &evmGenesis)
	evmGenesis.Params.EvmDenom = testconstants.ChainsCoinInfo[evmChainID].Denom
	genesisState[types.ModuleName] = app.AppCodec().MustMarshalJSON(&evmGenesis)

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
func SetupTestingApp(chainID string) func() (ibctesting.TestingApp, map[string]json.RawMessage) {
	return func() (ibctesting.TestingApp, map[string]json.RawMessage) {
		db := dbm.NewMemDB()
		app := NewExampleApp(
			log.NewNopLogger(),
			db, nil, true,
			simtestutil.NewAppOptionsWithFlagHome(defaultNodeHome),
			baseapp.SetChainID(chainID),
		)
		return app, app.DefaultGenesis()
	}
}
