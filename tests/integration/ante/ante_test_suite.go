package ante

import (
	"math"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/testutil/integration"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	consensustypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
)

const TestGasLimit uint64 = 100000

type AnteTestSuite struct { //nolint:revive
	suite.Suite

	create      network.CreateEvmApp
	options     []network.ConfigOption
	network     *network.UnitTestNetwork
	handler     grpc.Handler
	keyring     keyring.Keyring
	factory     factory.TxFactory
	clientCtx   client.Context
	anteHandler sdk.AnteHandler

	enableFeemarket bool
	baseFee         *sdkmath.LegacyDec
	enableLondonHF  bool
	evmParamsOption func(*evmtypes.Params)
}

func NewAnteTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *AnteTestSuite {
	suite := &AnteTestSuite{
		create:  create,
		options: options,
	}
	return suite
}

func (s *AnteTestSuite) SetupTest() {
	keys := keyring.New(2)

	customGenesis := network.CustomGenesisState{}
	feemarketGenesis := feemarkettypes.DefaultGenesisState()
	if s.enableFeemarket {
		feemarketGenesis.Params.EnableHeight = 1
		feemarketGenesis.Params.NoBaseFee = false
	} else {
		feemarketGenesis.Params.NoBaseFee = true
	}
	if s.baseFee != nil {
		feemarketGenesis.Params.BaseFee = *s.baseFee
	}
	customGenesis[feemarkettypes.ModuleName] = feemarketGenesis

	evmGenesis := evmtypes.DefaultGenesisState()

	if s.evmParamsOption != nil {
		s.evmParamsOption(&evmGenesis.Params)
	}
	customGenesis[evmtypes.ModuleName] = evmGenesis

	// set block max gas to be less than maxUint64
	cp := integration.DefaultConsensusParams
	cp.Block.MaxGas = 1000000000000000000
	customGenesis[consensustypes.ModuleName] = cp

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
	// set antehandler
	s.anteHandler = nw.App.GetAnteHandler()

	encodingConfig := nw.GetEncodingConfig()

	s.clientCtx = client.Context{}.WithTxConfig(encodingConfig.TxConfig)

	s.Require().NotNil(s.network.App.AppCodec())

	chainConfig := evmtypes.DefaultChainConfig(s.network.GetEIP155ChainID().Uint64())
	if !s.enableLondonHF {
		maxInt := sdkmath.NewInt(math.MaxInt64)
		chainConfig.LondonBlock = &maxInt
		chainConfig.ArrowGlacierBlock = &maxInt
		chainConfig.GrayGlacierBlock = &maxInt
		chainConfig.MergeNetsplitBlock = &maxInt
		chainConfig.ShanghaiTime = &maxInt
		chainConfig.CancunTime = &maxInt
		chainConfig.PragueTime = &maxInt
	}

	// get the denom and decimals set when initialized the chain
	// to set them again
	// when resetting the chain config
	denom := evmtypes.GetEVMCoinDenom()
	extendedDenom := evmtypes.GetEVMCoinExtendedDenom()
	displayDenom := evmtypes.GetEVMCoinDisplayDenom()
	decimals := evmtypes.GetEVMCoinDecimals()

	configurator := evmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	err := configurator.
		WithEVMCoinInfo(evmtypes.EvmCoinInfo{
			Denom:         denom,
			ExtendedDenom: extendedDenom,
			DisplayDenom:  displayDenom,
			Decimals:      decimals.Uint32(),
		}).
		Configure()
	s.Require().NoError(err)
}

func (s *AnteTestSuite) WithFeemarketEnabled(enabled bool) {
	s.enableFeemarket = enabled
}

func (s *AnteTestSuite) WithLondonHardForkEnabled(enabled bool) {
	s.enableLondonHF = enabled
}

func (s *AnteTestSuite) WithBaseFee(baseFee *sdkmath.LegacyDec) {
	s.baseFee = baseFee
}

func (s *AnteTestSuite) WithEvmParamsOptions(evmParamsOpts func(*evmtypes.Params)) {
	s.evmParamsOption = evmParamsOpts
}

func (s *AnteTestSuite) ResetEvmParamsOptions() {
	s.evmParamsOption = nil
}

func (s *AnteTestSuite) GetKeyring() keyring.Keyring {
	return s.keyring
}

func (s *AnteTestSuite) GetTxFactory() factory.TxFactory {
	return s.factory
}

func (s *AnteTestSuite) GetNetwork() *network.UnitTestNetwork {
	return s.network
}

func (s *AnteTestSuite) GetClientCtx() client.Context {
	return s.clientCtx
}

func (s *AnteTestSuite) GetAnteHandler() sdk.AnteHandler {
	return s.anteHandler
}

func (s *AnteTestSuite) CreateTestCosmosTxBuilder(gasPrice sdkmath.Int, denom string, msgs ...sdk.Msg) client.TxBuilder {
	txBuilder := s.GetClientCtx().TxConfig.NewTxBuilder()

	txBuilder.SetGasLimit(TestGasLimit)
	fees := &sdk.Coins{{Denom: denom, Amount: gasPrice.MulRaw(int64(TestGasLimit))}}
	txBuilder.SetFeeAmount(*fees)
	err := txBuilder.SetMsgs(msgs...)
	s.Require().NoError(err)
	return txBuilder
}

func (s *AnteTestSuite) CreateTestCosmosTxBuilderWithFees(fees sdk.Coins, msgs ...sdk.Msg) client.TxBuilder {
	txBuilder := s.GetClientCtx().TxConfig.NewTxBuilder()
	txBuilder.SetGasLimit(TestGasLimit)
	txBuilder.SetFeeAmount(fees)
	err := txBuilder.SetMsgs(msgs...)
	s.Require().NoError(err)
	return txBuilder
}
