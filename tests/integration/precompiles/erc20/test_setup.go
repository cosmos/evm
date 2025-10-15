package erc20

import (
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/precompiles/erc20"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
)

// PrecompileTestSuite is the implementation of the TestSuite interface for ERC20 precompile
// unit tests.
type PrecompileTestSuite struct {
	suite.Suite

	create    network.CreateEvmApp
	options   []network.ConfigOption
	bondDenom string
	// tokenDenom is the specific token denomination used in testing the ERC20 precompile.
	// This denomination is used to instantiate the precompile.
	tokenDenom  string
	network     *network.UnitTestNetwork
	factory     factory.TxFactory
	grpcHandler grpc.Handler
	keyring     testkeyring.Keyring

	precompile *erc20.Precompile

	// precompile2 is a second instance of the ERC20 precompile whose denom is bondDenom.
	precompile2 *erc20.Precompile
}

func NewPrecompileTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *PrecompileTestSuite {
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
	integrationNetwork := network.NewUnitTestNetwork(s.create, options...)
	grpcHandler := grpc.NewIntegrationHandler(integrationNetwork)
	txFactory := factory.New(integrationNetwork, grpcHandler)

	ctx := integrationNetwork.GetContext()
	sk := integrationNetwork.App.GetStakingKeeper()
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
	tokenPairID := s.network.App.GetErc20Keeper().GetDenomMap(s.network.GetContext(), bondDenom)
	tokenPair, found := s.network.App.GetErc20Keeper().GetTokenPair(s.network.GetContext(), tokenPairID)
	s.Require().True(found)
	s.precompile2 = erc20.NewPrecompile(tokenPair, s.network.App.GetBankKeeper(), s.network.App.GetErc20Keeper(), s.network.App.GetTransferKeeper())
}
