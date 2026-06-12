package precisebank

import (
	"github.com/stretchr/testify/suite"

	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const SEED = int64(42)

type KeeperIntegrationTestSuite struct {
	suite.Suite

	create  network.CreateEvmApp
	options []network.ConfigOption
	network *network.UnitTestNetwork
	factory factory.TxFactory
	keyring keyring.Keyring
}

func NewKeeperIntegrationTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *KeeperIntegrationTestSuite {
	return &KeeperIntegrationTestSuite{
		create:  create,
		options: options,
	}
}

func (s *KeeperIntegrationTestSuite) SetupTest() {
	s.SetupTestWithChainID(testconstants.SixDecimalsChainID)
}

func (s *KeeperIntegrationTestSuite) SetupTestWithChainID(chainID testconstants.ChainID) {
	s.keyring = keyring.New(2)

	options := []network.ConfigOption{
		network.WithChainID(chainID),
		network.WithPreFundedAccounts(s.keyring.GetAllAccAddrs()...),
	}
	options = append(options, s.options...)
	nw := network.NewUnitTestNetwork(s.create, options...)
	gh := grpc.NewIntegrationHandler(nw)
	tf := factory.New(nw, gh)

	s.network = nw
	s.factory = tf

	// Clear all fractional balances to ensure no leftover balances persist between tests
	s.network.App.GetPreciseBankKeeper().IterateFractionalBalances(s.network.GetContext(), func(addr sdk.AccAddress, bal sdkmath.Int) bool {
		s.network.App.GetPreciseBankKeeper().DeleteFractionalBalance(s.network.GetContext(), addr)
		return false
	})
}
