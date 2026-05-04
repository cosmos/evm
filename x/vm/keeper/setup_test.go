package keeper_test

import (
	"math"
	"testing"

	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/testutil/integration/os/factory"
	"github.com/cosmos/evm/testutil/integration/os/grpc"
	"github.com/cosmos/evm/testutil/integration/os/keyring"
	"github.com/cosmos/evm/testutil/integration/os/network"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type KeeperTestSuite struct {
	suite.Suite

	network *network.UnitTestNetwork
	handler grpc.Handler
	keyring keyring.Keyring
	factory factory.TxFactory

	enableFeemarket  bool
	enableLondonHF   bool
	mintFeeCollector bool
}

type UnitTestSuite struct {
	suite.Suite
}

var s *KeeperTestSuite

func TestKeeperTestSuite(t *testing.T) {
	s = new(KeeperTestSuite)
	s.enableFeemarket = false
	s.enableLondonHF = true
	suite.Run(t, s)

	// Run UnitTestSuite
	unitTestSuite := new(UnitTestSuite)
	suite.Run(t, unitTestSuite)
}

func (suite *KeeperTestSuite) SetupTest() {
	keys := keyring.New(2)
	// Set custom balance based on test params
	customGenesis := network.CustomGenesisState{}
	feemarketGenesis := feemarkettypes.DefaultGenesisState()
	if suite.enableFeemarket {
		feemarketGenesis.Params.EnableHeight = 1
		feemarketGenesis.Params.NoBaseFee = false
	} else {
		feemarketGenesis.Params.NoBaseFee = true
	}
	customGenesis[feemarkettypes.ModuleName] = feemarketGenesis

	if suite.mintFeeCollector {
		// Mint coins to fee collector for gas refunds
		baseFee := feemarketGenesis.Params.BaseFee.TruncateInt()
		gasUsed := sdkmath.NewIntFromUint64(ethparams.TxGas)
		refundBuffer := sdkmath.NewIntFromUint64(ethparams.TxGas - 1)
		requiredBalance := gasUsed.Mul(baseFee).Add(refundBuffer)

		coins := sdk.NewCoins(sdk.NewCoin(evmtypes.GetEVMCoinExtendedDenom(), requiredBalance))
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

	nw := network.NewUnitTestNetwork(
		network.WithPreFundedAccounts(keys.GetAllAccAddrs()...),
		network.WithCustomGenesis(customGenesis),
	)
	gh := grpc.NewIntegrationHandler(nw)
	tf := factory.New(nw, gh)

	suite.network = nw
	suite.factory = tf
	suite.handler = gh
	suite.keyring = keys

	chainConfig := evmtypes.DefaultChainConfig(suite.network.GetEIP155ChainID().Uint64())
	if !suite.enableLondonHF {
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
	denom := evmtypes.GetEVMCoinDenom()                 //nolint:staticcheck
	extendedDenom := evmtypes.GetEVMCoinExtendedDenom() //nolint:staticcheck
	decimals := evmtypes.GetEVMCoinDecimals()           //nolint:staticcheck

	configurator := evmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	err := configurator.
		WithChainConfig(chainConfig).
		WithEVMCoinInfo(evmtypes.EvmCoinInfo{
			Denom:         denom,
			ExtendedDenom: extendedDenom,
			Decimals:      decimals,
		}).
		Configure()
	suite.Require().NoError(err)
}
