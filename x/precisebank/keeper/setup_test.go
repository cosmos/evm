package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/os/factory"
	"github.com/cosmos/evm/testutil/integration/os/grpc"
	"github.com/cosmos/evm/testutil/integration/os/keyring"
	"github.com/cosmos/evm/testutil/integration/os/network"
	evmtypes "github.com/cosmos/evm/x/vm/types"
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

func (suite *KeeperIntegrationTestSuite) SetupTestWithChainID(chainID testconstants.ChainID) {
	suite.keyring = keyring.New(2)

	// Reset evm config here for the standard case
	configurator := evmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	err := configurator.
		WithEVMCoinInfo(testconstants.ExampleChainCoinInfo[chainID]).
		Configure()
	if err != nil {
		return
	}

	nw := network.NewUnitTestNetwork(
		network.WithChainID(chainID),
		network.WithPreFundedAccounts(suite.keyring.GetAllAccAddrs()...),
	)
	gh := grpc.NewIntegrationHandler(nw)
	tf := factory.New(nw, gh)

	suite.network = nw
	suite.factory = tf
}
