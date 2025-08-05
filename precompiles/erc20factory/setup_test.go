package erc20factory_test

import (
	"github.com/cosmos/evm/precompiles/erc20factory"
	"github.com/cosmos/evm/testutil/integration/os/factory"
	"github.com/cosmos/evm/testutil/integration/os/grpc"
	testkeyring "github.com/cosmos/evm/testutil/integration/os/keyring"
	"github.com/cosmos/evm/testutil/integration/os/network"
	"github.com/stretchr/testify/suite"
)

// PrecompileTestSuite is the implementation of the TestSuite interface for ERC20 Factory precompile
// unit tests.
type PrecompileTestSuite struct {
	suite.Suite

	create      network.UnitTestNetwork
	options     []network.ConfigOption
	bondDenom   string
	network     *network.UnitTestNetwork
	factory     factory.TxFactory
	grpcHandler grpc.Handler
	keyring     testkeyring.Keyring

	precompile *erc20factory.Precompile
}

func NewPrecompileTestSuite(create network.UnitTestNetwork, options ...network.ConfigOption) *PrecompileTestSuite {
	return &PrecompileTestSuite{
		create:  create,
		options: options,
	}
}

func (s *PrecompileTestSuite) SetupTest() {
	keyring := testkeyring.New(2)
	options := []network.ConfigOption{
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	}
	options = append(options, s.options...)
	integrationNetwork := network.NewUnitTestNetwork(options...)
	grpcHandler := grpc.NewIntegrationHandler(integrationNetwork)
	txFactory := factory.New(integrationNetwork, grpcHandler)

	ctx := integrationNetwork.GetContext()
	sk := integrationNetwork.App.StakingKeeper
	bondDenom, err := sk.BondDenom(ctx)
	s.Require().NoError(err)
	s.Require().NotEmpty(bondDenom, "bond denom cannot be empty")

	s.bondDenom = bondDenom
	s.factory = txFactory
	s.grpcHandler = grpcHandler
	s.keyring = keyring
	s.network = integrationNetwork

	// Instantiate the precompile with an exemplary token denomination.
	//
	// NOTE: This has to be done AFTER assigning the suite fields.
	s.precompile = s.setupERC20FactoryPrecompile()
}
