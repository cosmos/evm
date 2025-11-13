package bank

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/cosmos-sdk/baseapp"
	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/bank"
)

func TestBankPrecompileTestSuite(t *testing.T) {
	create := func(chainID string, evmChainID uint64, customBaseAppOptions ...func(*baseapp.BaseApp)) evm.BankPrecompileApp {
		return integration.CreateEvmd(chainID, evmChainID, customBaseAppOptions...).(evm.BankPrecompileApp)
	}
	s := bank.NewPrecompileTestSuite(create)
	suite.Run(t, s)
}

func TestBankPrecompileIntegrationTestSuite(t *testing.T) {
	create := func(chainID string, evmChainID uint64, customBaseAppOptions ...func(*baseapp.BaseApp)) evm.BankPrecompileApp {
		return integration.CreateEvmd(chainID, evmChainID, customBaseAppOptions...).(evm.BankPrecompileApp)
	}
	bank.TestIntegrationSuite(t, create)
}
