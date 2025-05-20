package testutils

import (
	"math"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/ante"
	evmante "github.com/cosmos/evm/ante/evm"
	chainante "github.com/cosmos/evm/evmd/ante"
	chainutil "github.com/cosmos/evm/evmd/testutil"
	"github.com/cosmos/evm/testutil/integration/os/factory"
	"github.com/cosmos/evm/testutil/integration/os/grpc"
	"github.com/cosmos/evm/testutil/integration/os/keyring"
	"github.com/cosmos/evm/testutil/integration/os/network"
	"github.com/cosmos/evm/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	consensustypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
)

type AnteTestSuite struct {
	suite.Suite

	network   *network.UnitTestNetwork
	handler   grpc.Handler
	keyring   keyring.Keyring
	factory   factory.TxFactory
	clientCtx client.Context

	anteHandler     sdk.AnteHandler
	enableFeemarket bool
	baseFee         *sdkmath.LegacyDec
	enableLondonHF  bool
	evmParamsOption func(*evmtypes.Params)
}

const TestGasLimit uint64 = 100000

func (suite *AnteTestSuite) SetupTest() {
	keys := keyring.New(2)

	customGenesis := network.CustomGenesisState{}
	feemarketGenesis := feemarkettypes.DefaultGenesisState()
	if suite.enableFeemarket {
		feemarketGenesis.Params.EnableHeight = 1
		feemarketGenesis.Params.NoBaseFee = false
	} else {
		feemarketGenesis.Params.NoBaseFee = true
	}
	if suite.baseFee != nil {
		feemarketGenesis.Params.BaseFee = *suite.baseFee
	}
	customGenesis[feemarkettypes.ModuleName] = feemarketGenesis

	evmGenesis := evmtypes.DefaultGenesisState()

	if suite.evmParamsOption != nil {
		suite.evmParamsOption(&evmGenesis.Params)
	}
	customGenesis[evmtypes.ModuleName] = evmGenesis

	// set block max gas to be less than maxUint64
	cp := chainutil.DefaultConsensusParams
	cp.Block.MaxGas = 1000000000000000000
	customGenesis[consensustypes.ModuleName] = cp

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

	encodingConfig := nw.GetEncodingConfig()

	suite.clientCtx = client.Context{}.WithTxConfig(encodingConfig.TxConfig)

	suite.Require().NotNil(suite.network.App.AppCodec())

	chainConfig := evmtypes.DefaultChainConfig(suite.network.GetChainID())
	if !suite.enableLondonHF {
		maxInt := sdkmath.NewInt(math.MaxInt64)
		chainConfig.LondonBlock = &maxInt
		chainConfig.ArrowGlacierBlock = &maxInt
		chainConfig.GrayGlacierBlock = &maxInt
		chainConfig.MergeNetsplitBlock = &maxInt
	}

	// get the denom and decimals set when initialized the chain
	// to set them again
	// when resetting the chain config
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

	anteHandler := chainante.NewAnteHandler(chainante.HandlerOptions{
		Cdc:                    suite.network.App.AppCodec(),
		AccountKeeper:          suite.network.App.AccountKeeper,
		BankKeeper:             suite.network.App.BankKeeper,
		EvmKeeper:              suite.network.App.EVMKeeper,
		FeegrantKeeper:         suite.network.App.FeeGrantKeeper,
		IBCKeeper:              suite.network.App.IBCKeeper,
		FeeMarketKeeper:        suite.network.App.FeeMarketKeeper,
		SignModeHandler:        encodingConfig.TxConfig.SignModeHandler(),
		SigGasConsumer:         ante.SigVerificationGasConsumer,
		ExtensionOptionChecker: types.HasDynamicFeeExtensionOption,
		TxFeeChecker:           evmante.NewDynamicFeeChecker(suite.network.App.FeeMarketKeeper),
	})

	suite.anteHandler = anteHandler
}

func (suite *AnteTestSuite) WithFeemarketEnabled(enabled bool) {
	suite.enableFeemarket = enabled
}

func (suite *AnteTestSuite) WithLondonHardForkEnabled(enabled bool) {
	suite.enableLondonHF = enabled
}

func (suite *AnteTestSuite) WithBaseFee(baseFee *sdkmath.LegacyDec) {
	suite.baseFee = baseFee
}

func (suite *AnteTestSuite) WithEvmParamsOptions(evmParamsOpts func(*evmtypes.Params)) {
	suite.evmParamsOption = evmParamsOpts
}

func (suite *AnteTestSuite) ResetEvmParamsOptions() {
	suite.evmParamsOption = nil
}

func (suite *AnteTestSuite) GetKeyring() keyring.Keyring {
	return suite.keyring
}

func (suite *AnteTestSuite) GetTxFactory() factory.TxFactory {
	return suite.factory
}

func (suite *AnteTestSuite) GetNetwork() *network.UnitTestNetwork {
	return suite.network
}

func (suite *AnteTestSuite) GetClientCtx() client.Context {
	return suite.clientCtx
}

func (suite *AnteTestSuite) GetAnteHandler() sdk.AnteHandler {
	return suite.anteHandler
}
