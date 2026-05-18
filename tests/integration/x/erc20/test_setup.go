package erc20

import (
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"
	"github.com/cosmos/evm/x/erc20/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type KeeperTestSuite struct {
	suite.Suite

	create  network.CreateEvmApp
	options []network.ConfigOption
	network *network.UnitTestNetwork
	handler grpc.Handler
	keyring keyring.Keyring
	factory factory.TxFactory

	queryClient types.QueryClient

	mintFeeCollector bool
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

	if s.mintFeeCollector {
		baseDenom, err := sdk.GetBaseDenom()
		s.Require().NoError(err, "failed to get base denom")

		// mint some coin to fee collector
		coins := sdk.NewCoins(sdk.NewCoin(baseDenom, sdkmath.NewInt(int64(params.TxGas)-1)))
		balances := []banktypes.Balance{
			{
				Address: authtypes.NewModuleAddress(authtypes.FeeCollectorName).String(),
				Coins:   coins,
			},
		}
		bankGenesis := banktypes.DefaultGenesisState()
		bankGenesis.Balances = balances
		customGenesis[banktypes.ModuleName] = bankGenesis
	}

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
	s.queryClient = nw.GetERC20Client()
}
