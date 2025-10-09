package staking

import (
	"github.com/stretchr/testify/suite"

	evmaddress "github.com/cosmos/evm/encoding/address"
	"github.com/cosmos/evm/precompiles/staking"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

const InitialTestBalance = 1000000000000000000 // 1 atom

type PrecompileTestSuite struct {
	suite.Suite

	create      network.CreateEvmApp
	options     []network.ConfigOption
	network     *network.UnitTestNetwork
	factory     factory.TxFactory
	grpcHandler grpc.Handler
	keyring     testkeyring.Keyring

	bondDenom     string
	precompile    *staking.Precompile
	customGenesis bool
}

func NewPrecompileTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *PrecompileTestSuite {
	return &PrecompileTestSuite{
		create:  create,
		options: options,
	}
}

func (s *PrecompileTestSuite) SetupTest() {
	keyring := testkeyring.New(2)
	customGenesis := network.CustomGenesisState{}
	// mint some coin to fee collector
	coins := sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(InitialTestBalance)))
	balances := []banktypes.Balance{
		{
			Address: authtypes.NewModuleAddress(authtypes.FeeCollectorName).String(),
			Coins:   coins,
		},
	}
	bankGenesis := banktypes.DefaultGenesisState()
	bankGenesis.Balances = balances
	customGenesis[banktypes.ModuleName] = bankGenesis
	opts := []network.ConfigOption{
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	}
	if s.customGenesis {
		opts = append(opts, network.WithCustomGenesis(customGenesis))
	}
	opts = append(opts, s.options...)
	nw := network.NewUnitTestNetwork(s.create, opts...)
	grpcHandler := grpc.NewIntegrationHandler(nw)
	txFactory := factory.New(nw, grpcHandler)

	ctx := nw.GetContext()
	sk := nw.App.GetStakingKeeper()
	bondDenom, err := sk.BondDenom(ctx)
	if err != nil {
		panic(err)
	}

	s.bondDenom = bondDenom
	s.factory = txFactory
	s.grpcHandler = grpcHandler
	s.keyring = keyring
	s.network = nw

	s.precompile = staking.NewPrecompile(
		*s.network.App.GetStakingKeeper(),
		stakingkeeper.NewMsgServerImpl(s.network.App.GetStakingKeeper()),
		stakingkeeper.NewQuerier(s.network.App.GetStakingKeeper()),
		s.network.App.GetBankKeeper(),
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
	)
}
