package integration

import (
	"encoding/json"
	"testing"

	"github.com/cosmos/evm/server/config"
	"github.com/cosmos/evm/tests/integration/x/vm"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func BenchmarkGasEstimation(b *testing.B) {
	// Setup benchmark test environment
	keys := keyring.New(2)
	// Set custom balance based on test params
	customGenesis := network.CustomGenesisState{}
	feemarketGenesis := feemarkettypes.DefaultGenesisState()
	feemarketGenesis.Params.NoBaseFee = true
	customGenesis[feemarkettypes.ModuleName] = feemarketGenesis
	opts := []network.ConfigOption{
		network.WithPreFundedAccounts(keys.GetAllAccAddrs()...),
		network.WithCustomGenesis(customGenesis),
	}
	nw := network.NewUnitTestNetwork(CreateEvmd, opts...)
	// gh := grpc.NewIntegrationHandler(nw)
	// tf := factory.New(nw, gh)

	chainConfig := types.DefaultChainConfig(nw.GetEIP155ChainID().Uint64())
	// get the denom and decimals set on chain initialization
	// because we'll need to set them again when resetting the chain config
	denom := types.GetEVMCoinDenom()
	extendedDenom := types.GetEVMCoinExtendedDenom()
	displayDenom := types.GetEVMCoinDisplayDenom()
	decimals := types.GetEVMCoinDecimals()

	configurator := types.NewEVMConfigurator()
	configurator.ResetTestConfig()
	err := types.SetChainConfig(chainConfig)
	require.NoError(b, err)
	err = configurator.
		WithEVMCoinInfo(types.EvmCoinInfo{
			Denom:         denom,
			ExtendedDenom: extendedDenom,
			DisplayDenom:  displayDenom,
			Decimals:      decimals.Uint32(),
		}).
		Configure()
	require.NoError(b, err)

	// Use simple transaction args for consistent benchmarking
	args := types.TransactionArgs{
		To: &common.Address{},
	}

	marshalArgs, err := json.Marshal(args)
	require.NoError(b, err)

	req := types.EthCallRequest{
		Args:            marshalArgs,
		GasCap:          config.DefaultGasCap,
		ProposerAddress: nw.GetContext().BlockHeader().ProposerAddress,
	}

	// Reset timer to exclude setup time
	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		_, err := nw.GetEvmClient().EstimateGas(
			nw.GetContext(),
			&req,
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestKeeperTestSuite(t *testing.T) {
	s := vm.NewKeeperTestSuite(CreateEvmd)
	s.EnableFeemarket = false
	s.EnableLondonHF = true
	suite.Run(t, s)
}

func TestNestedEVMExtensionCallSuite(t *testing.T) {
	s := vm.NewNestedEVMExtensionCallSuite(CreateEvmd)
	suite.Run(t, s)
}

func TestGenesisTestSuite(t *testing.T) {
	s := vm.NewGenesisTestSuite(CreateEvmd)
	suite.Run(t, s)
}

func TestVmAnteTestSuite(t *testing.T) {
	s := vm.NewEvmAnteTestSuite(CreateEvmd)
	suite.Run(t, s)
}

func TestIterateContracts(t *testing.T) {
	vm.TestIterateContracts(t, CreateEvmd)
}
