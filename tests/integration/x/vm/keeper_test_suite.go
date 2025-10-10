package vm

import (
	"math"

	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type KeeperTestSuite struct {
	suite.Suite

	Network *network.UnitTestNetwork
	Create  network.CreateEvmApp
	Options []network.ConfigOption
	Handler grpc.Handler
	Keyring keyring.Keyring
	Factory factory.TxFactory

	EnableFeemarket  bool
	EnableLondonHF   bool
	MintFeeCollector bool
}

func NewKeeperTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *KeeperTestSuite {
	return &KeeperTestSuite{
		Create:          create,
		Options:         options,
		EnableFeemarket: false,
		EnableLondonHF:  true,
	}
}

func (s *KeeperTestSuite) SetupTest() {
	keys := keyring.New(2)
	// Set custom balance based on test params
	customGenesis := network.CustomGenesisState{}
	feemarketGenesis := feemarkettypes.DefaultGenesisState()
	if s.EnableFeemarket {
		feemarketGenesis.Params.EnableHeight = 1
		feemarketGenesis.Params.NoBaseFee = false
	} else {
		feemarketGenesis.Params.NoBaseFee = true
	}
	customGenesis[feemarkettypes.ModuleName] = feemarketGenesis

	if s.MintFeeCollector {
		// mint some coin to fee collector
		coins := sdk.NewCoins(sdk.NewCoin(evmtypes.GetEVMCoinDenom(), sdkmath.NewInt(int64(params.TxGas)-1)))
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

	if s.Options == nil {
		s.Options = []network.ConfigOption{}
	}
	opts := []network.ConfigOption{
		network.WithPreFundedAccounts(keys.GetAllAccAddrs()...),
		network.WithCustomGenesis(customGenesis),
	}
	opts = append(opts, s.Options...)
	nw := network.NewUnitTestNetwork(s.Create, opts...)
	gh := grpc.NewIntegrationHandler(nw)
	tf := factory.New(nw, gh)

	s.Network = nw
	s.Factory = tf
	s.Handler = gh
	s.Keyring = keys

	chainConfig := evmtypes.DefaultChainConfig(s.Network.GetEIP155ChainID().Uint64())
	if !s.EnableLondonHF {
		maxInt := sdkmath.NewInt(math.MaxInt64)
		chainConfig.LondonBlock = &maxInt
		chainConfig.ArrowGlacierBlock = &maxInt
		chainConfig.GrayGlacierBlock = &maxInt
		chainConfig.MergeNetsplitBlock = &maxInt
		chainConfig.ShanghaiTime = &maxInt
		chainConfig.CancunTime = &maxInt
		chainConfig.PragueTime = &maxInt
	}
	// get the denom and decimals set on chain initialization
	// because we'll need to set them again when resetting the chain config
	denom := evmtypes.GetEVMCoinDenom()
	extendedDenom := evmtypes.GetEVMCoinExtendedDenom()
	displayDenom := evmtypes.GetEVMCoinDisplayDenom()
	decimals := evmtypes.GetEVMCoinDecimals()

	configurator := evmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	err := evmtypes.SetChainConfig(chainConfig)
	s.Require().NoError(err)
	err = configurator.
		WithEVMCoinInfo(evmtypes.EvmCoinInfo{
			Denom:         denom,
			ExtendedDenom: extendedDenom,
			DisplayDenom:  displayDenom,
			Decimals:      decimals.Uint32(),
		}).
		Configure()
	s.Require().NoError(err)
}
