package bank

import (
	"testing"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/precompiles/bank"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestBankPrecompileTestSuite(t *testing.T) {
	configs := []bank.PrecompileSuiteConfig{
		{
			Name:   "default",
			Create: testapp.ToEvmAppCreator[evm.BankPrecompileApp](CreateEvmd, "evm.BankPrecompileApp"),
		},
		{
			Name:   "blockstm",
			Create: testapp.ToEvmAppCreator[evm.BankPrecompileApp](CreateEvmdWithBlockSTM, "evm.BankPrecompileApp"),
		},
	}

	bank.RunPrecompileTestSuites(t, configs...)
}

func TestBankPrecompileIntegrationTestSuite(t *testing.T) {
	configs := []bank.IntegrationSuiteConfig{
		{
			Name:   "default",
			Create: testapp.ToEvmAppCreator[evm.BankPrecompileApp](CreateEvmd, "evm.BankPrecompileApp"),
		},
		{
			Name:   "blockstm",
			Create: testapp.ToEvmAppCreator[evm.BankPrecompileApp](CreateEvmdWithBlockSTM, "evm.BankPrecompileApp"),
		},
	}

	bank.TestIntegrationSuite(t, configs...)
}
