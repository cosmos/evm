package erc20_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	erc20precompile "github.com/cosmos/evm/precompiles/erc20"
	"github.com/cosmos/evm/testutil/integration/os/factory"
	"github.com/cosmos/evm/testutil/integration/os/grpc"
	testkeyring "github.com/cosmos/evm/testutil/integration/os/keyring"
	"github.com/cosmos/evm/testutil/integration/os/network"
)

var s *PrecompileTestSuite

// PrecompileTestSuite is the implementation of the TestSuite interface for ERC20 precompile
// unit tests.
type PrecompileTestSuite struct {
	suite.Suite

	bondDenom string
	// tokenDenom is the specific token denomination used in testing the ERC20 precompile.
	// This denomination is used to instantiate the precompile.
	tokenDenom  string
	network     *network.UnitTestNetwork
	factory     factory.TxFactory
	grpcHandler grpc.Handler
	keyring     testkeyring.Keyring

	precompile *erc20precompile.Precompile

	// precompile2 is a second instance of the ERC20 precompile whose denom is bondDenom.
	precompile2 *erc20precompile.Precompile
}

func TestPrecompileTestSuite(t *testing.T) {
	s = new(PrecompileTestSuite)
	suite.Run(t, s)
}

func (s *PrecompileTestSuite) SetupTest() {
	keyring := testkeyring.New(2)
	integrationNetwork := network.NewUnitTestNetwork(
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	)
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
	s.tokenDenom = "xmpl"
	s.precompile, err = s.setupERC20Precompile(s.tokenDenom)
	s.Require().NoError(err)

	// Instantiate the precompile2 with the bond denom (the token pair was already set up in genesis).
	tokenPairID := s.network.App.Erc20Keeper.GetDenomMap(s.network.GetContext(), bondDenom)
	tokenPair, found := s.network.App.Erc20Keeper.GetTokenPair(s.network.GetContext(), tokenPairID)
	s.Require().True(found)
	s.precompile2, err = erc20precompile.NewPrecompile(tokenPair, s.network.App.BankKeeper, s.network.App.Erc20Keeper, s.network.App.TransferKeeper)
	s.Require().NoError(err)
}
