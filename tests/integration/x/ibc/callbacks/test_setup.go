package callbacks

import (
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"
)

type KeeperTestSuite struct {
	suite.Suite

	create  network.CreateEvmApp
	options []network.ConfigOption
	network *network.UnitTestNetwork
	handler grpc.Handler
	keyring keyring.Keyring
	factory factory.TxFactory
}

func NewKeeperTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *KeeperTestSuite {
	return &KeeperTestSuite{
		create:  create,
		options: options,
	}
}

func (s *KeeperTestSuite) SetupTest() {
	keys := keyring.New(2)
	// Set custom balance based on test params
	customGenesis := network.CustomGenesisState{}

	options := []network.ConfigOption{
		network.WithPreFundedAccounts(keys.GetAllAccAddrs()...),
		network.WithCustomGenesis(customGenesis),
	}
	options = append(options, s.options...)
	nw := network.NewUnitTestNetwork(s.create, options...)
	gh := grpc.NewIntegrationHandler(nw)
	tf := factory.New(nw, gh)

	s.network = nw
	s.factory = tf
	s.handler = gh
	s.keyring = keys
}
