package distribution

import (
	"github.com/stretchr/testify/suite"

	evmaddress "github.com/cosmos/evm/encoding/address"
	"github.com/cosmos/evm/precompiles/distribution"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

type PrecompileTestSuite struct {
	suite.Suite

	create      network.CreateEvmApp
	options     []network.ConfigOption
	network     *network.UnitTestNetwork
	factory     factory.TxFactory
	grpcHandler grpc.Handler
	keyring     testkeyring.Keyring

	precompile           *distribution.Precompile
	bondDenom            string
	baseDenom            string
	otherDenoms          []string
	validatorsKeys       []testkeyring.Key
	withValidatorSlashes bool
}

func NewPrecompileTestSuite(
	create network.CreateEvmApp,
	options ...network.ConfigOption,
) *PrecompileTestSuite {
	return &PrecompileTestSuite{
		create:  create,
		options: options,
	}
}

func (s *PrecompileTestSuite) SetupTest() {
	keyring := testkeyring.New(2)
	s.validatorsKeys = generateKeys(3)
	customGen := network.CustomGenesisState{}

	// mint some coin to fee collector
	coins := sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(1000000000000000000)))
	balances := []banktypes.Balance{
		{
			Address: authtypes.NewModuleAddress(authtypes.FeeCollectorName).String(),
			Coins:   coins,
		},
	}
	bankGenesis := banktypes.DefaultGenesisState()
	bankGenesis.Balances = balances
	customGen[banktypes.ModuleName] = bankGenesis

	// set some slashing events for integration test
	distrGen := distrtypes.DefaultGenesisState()
	if s.withValidatorSlashes {
		distrGen.ValidatorSlashEvents = []distrtypes.ValidatorSlashEventRecord{
			{
				ValidatorAddress:    sdk.ValAddress(s.validatorsKeys[0].Addr.Bytes()).String(),
				Height:              0,
				Period:              1,
				ValidatorSlashEvent: distrtypes.NewValidatorSlashEvent(1, math.LegacyNewDecWithPrec(5, 2)),
			},
			{
				ValidatorAddress:    sdk.ValAddress(s.validatorsKeys[0].Addr.Bytes()).String(),
				Height:              1,
				Period:              1,
				ValidatorSlashEvent: distrtypes.NewValidatorSlashEvent(1, math.LegacyNewDecWithPrec(5, 2)),
			},
		}
	}
	customGen[distrtypes.ModuleName] = distrGen

	// set non-zero inflation for rewards to accrue (use defaults from SDK for values)
	mintGen := minttypes.DefaultGenesisState()
	mintGen.Params.MintDenom = testconstants.ExampleAttoDenom
	customGen[minttypes.ModuleName] = mintGen

	operatorsAddr := make([]sdk.AccAddress, 3)
	for i, k := range s.validatorsKeys {
		operatorsAddr[i] = k.AccAddr
	}

	s.otherDenoms = []string{
		testconstants.OtherCoinDenoms[0],
		testconstants.OtherCoinDenoms[1],
	}

	options := []network.ConfigOption{
		network.WithOtherDenoms(testconstants.OtherCoinDenoms),
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
		network.WithOtherDenoms(s.otherDenoms),
		network.WithCustomGenesis(customGen),
		network.WithValidatorOperators(operatorsAddr),
	}
	options = append(options, s.options...)
	nw := network.NewUnitTestNetwork(s.create, options...)
	grpcHandler := grpc.NewIntegrationHandler(nw)
	txFactory := factory.New(nw, grpcHandler)

	ctx := nw.GetContext()
	sk := nw.App.GetStakingKeeper()
	bondDenom, err := sk.BondDenom(ctx)
	if err != nil {
		panic(err)
	}

	s.bondDenom = bondDenom
	// TODO: check if this is correct?
	s.baseDenom = evmtypes.GetEVMCoinDenom()

	s.factory = txFactory
	s.grpcHandler = grpcHandler
	s.keyring = keyring
	s.network = nw
	s.precompile = distribution.NewPrecompile(
		s.network.App.GetDistrKeeper(),
		distrkeeper.NewMsgServerImpl(s.network.App.GetDistrKeeper()),
		distrkeeper.NewQuerier(s.network.App.GetDistrKeeper()),
		*s.network.App.GetStakingKeeper(),
		s.network.App.GetBankKeeper(),
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
	)
}
