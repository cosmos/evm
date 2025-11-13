package bank

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	bank2 "github.com/cosmos/evm/precompiles/bank"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/integration/evm/utils"
	testkeyring "github.com/cosmos/evm/testutil/keyring"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

// PrecompileTestSuite is the implementation of the TestSuite interface for ERC20 precompile
// unit tests.
type PrecompileTestSuite struct {
	suite.Suite

	create                  network.CreateEvmApp
	bondDenom, tokenDenom   string
	cosmosEVMAddr, xmplAddr common.Address

	// tokenDenom is the specific token denomination used in testing the ERC20 precompile.
	// This denomination is used to instantiate the precompile.
	network     *network.UnitTestNetwork
	factory     factory.TxFactory
	grpcHandler grpc.Handler
	keyring     testkeyring.Keyring

	precompile *bank2.Precompile
}

func NewPrecompileTestSuite(create network.CreateEvmApp) *PrecompileTestSuite {
	return &PrecompileTestSuite{
		create: create,
	}
}

func (s *PrecompileTestSuite) SetupTest() sdk.Context {
	s.tokenDenom = xmplDenom

	keyring := testkeyring.New(2)
	genesis := utils.CreateGenesisWithTokenPairs(keyring)
	unitNetwork := network.NewUnitTestNetwork(
		s.create,
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
		network.WithCustomGenesis(genesis),
		network.WithOtherDenoms([]string{s.tokenDenom}),
	)
	grpcHandler := grpc.NewIntegrationHandler(unitNetwork)
	txFactory := factory.New(unitNetwork, grpcHandler)

	ctx := unitNetwork.GetContext()
	sk := unitNetwork.App.GetStakingKeeper()
	bondDenom, err := sk.BondDenom(ctx)
	s.Require().NoError(err, "failed to get bond denom")
	s.Require().NotEmpty(bondDenom, "bond denom cannot be empty")

	s.bondDenom = bondDenom
	s.factory = txFactory
	s.grpcHandler = grpcHandler
	s.keyring = keyring
	s.network = unitNetwork

	tokenPairID := s.network.App.GetErc20Keeper().GetTokenPairID(s.network.GetContext(), s.bondDenom)
	tokenPair, found := s.network.App.GetErc20Keeper().GetTokenPair(s.network.GetContext(), tokenPairID)
	s.Require().True(found)
	s.cosmosEVMAddr = common.HexToAddress(tokenPair.Erc20Address)

	s.cosmosEVMAddr = tokenPair.GetERC20Contract()

	// Mint and register a second coin for testing purposes
	err = s.network.App.GetBankKeeper().MintCoins(s.network.GetContext(), minttypes.ModuleName, sdk.Coins{{Denom: "xmpl", Amount: math.NewInt(1e18)}})
	s.Require().NoError(err)

	tokenPairID = s.network.App.GetErc20Keeper().GetTokenPairID(s.network.GetContext(), s.tokenDenom)
	tokenPair, found = s.network.App.GetErc20Keeper().GetTokenPair(s.network.GetContext(), tokenPairID)
	s.Require().True(found)
	s.xmplAddr = common.HexToAddress(tokenPair.Erc20Address)

	s.xmplAddr = tokenPair.GetERC20Contract()

	s.precompile = s.setupBankPrecompile()
	return ctx
}
