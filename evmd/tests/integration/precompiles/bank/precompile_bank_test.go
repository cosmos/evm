package bank

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/appbuilder"
	banktests "github.com/cosmos/evm/tests/integration/precompiles/bank"
	testapp "github.com/cosmos/evm/testutil/app"

	"github.com/cosmos/cosmos-sdk/baseapp"
)

func TestBankPrecompileTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.BankPrecompileApp](
		func(chainID string, evmChainID uint64, customBaseAppOptions ...func(*baseapp.BaseApp)) evm.EvmApp {
			return appbuilder.CreateEvmdWithProfile(appbuilder.FullPrecompiles, chainID, evmChainID, customBaseAppOptions...)
		},
		"evm.BankPrecompileApp",
	)
	s := banktests.NewPrecompileTestSuite(create)
	suite.Run(t, s)
}

func TestBankPrecompileIntegrationTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.BankPrecompileApp](
		func(chainID string, evmChainID uint64, customBaseAppOptions ...func(*baseapp.BaseApp)) evm.EvmApp {
			return appbuilder.CreateEvmdWithProfile(appbuilder.FullPrecompiles, chainID, evmChainID, customBaseAppOptions...)
		},
		"evm.BankPrecompileApp",
	)
	banktests.TestIntegrationSuite(t, create)
}
