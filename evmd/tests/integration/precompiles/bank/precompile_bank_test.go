package bank

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/cosmos-sdk/baseapp"
	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/bank"
	testapp "github.com/cosmos/evm/testutil/app"
)

func bankAppFactory() func(string, uint64, ...func(*baseapp.BaseApp)) evm.EvmApp {
	return testapp.WrapToEvmApp[evm.BankPrecompileApp](integration.CreateEvmd, "evm.BankPrecompileApp")
}

func TestBankPrecompileTestSuite(t *testing.T) {
	s := bank.NewPrecompileTestSuite(bankAppFactory())
	suite.Run(t, s)
}

func TestBankPrecompileIntegrationTestSuite(t *testing.T) {
	bank.TestIntegrationSuite(t, bankAppFactory())
}
