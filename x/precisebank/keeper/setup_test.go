package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/os/factory"
	"github.com/cosmos/evm/testutil/integration/os/grpc"
	"github.com/cosmos/evm/testutil/integration/os/keyring"
	"github.com/cosmos/evm/testutil/integration/os/network"
)

const SEED = int64(42)

type KeeperIntegrationTestSuite struct {
	suite.Suite

	network *network.UnitTestNetwork
	factory factory.TxFactory
	keyring keyring.Keyring
}

func TestKeeperIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperIntegrationTestSuite))
}

func (suite *KeeperIntegrationTestSuite) SetupTest() {
	suite.SetupTestWithChainID(testconstants.SixDecimalsChainID)
}

func (suite *KeeperIntegrationTestSuite) SetupTestWithChainID(chainID string) {
	suite.keyring = keyring.New(2)

	nw := network.NewUnitTestNetwork(
		network.WithChainID(chainID),
		network.WithPreFundedAccounts(suite.keyring.GetAllAccAddrs()...),
	)
	gh := grpc.NewIntegrationHandler(nw)
	tf := factory.New(nw, gh)

	suite.network = nw
	suite.factory = tf
}
