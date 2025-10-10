package vm

import (
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
)

// GenesisTestSuite defines a testify suite for genesis integration tests.
type GenesisTestSuite struct {
	suite.Suite

	create  network.CreateEvmApp
	options []network.ConfigOption
	keyring testkeyring.Keyring
	network *network.UnitTestNetwork
	handler grpc.Handler
	factory factory.TxFactory
}

func NewGenesisTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *GenesisTestSuite {
	return &GenesisTestSuite{
		create:  create,
		options: options,
	}
}

// SetupTest resets state before each test method
func (s *GenesisTestSuite) SetupTest() {
	// initialize a fresh network, keyring, handler, and factory
	s.keyring = testkeyring.New(1)
	if s.options == nil {
		s.options = []network.ConfigOption{}
	}

	customGenesis := network.CustomGenesisState{}

	opts := []network.ConfigOption{
		network.WithPreFundedAccounts(s.keyring.GetAllAccAddrs()...),
		network.WithCustomGenesis(customGenesis),
	}
	opts = append(opts, s.options...)
	s.network = network.NewUnitTestNetwork(s.create, opts...)
	s.handler = grpc.NewIntegrationHandler(s.network)
	s.factory = factory.New(s.network, s.handler)
}
