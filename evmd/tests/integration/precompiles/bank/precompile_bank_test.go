package bank

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/bank"
)

func TestBankPrecompileTestSuite(t *testing.T) {
	s := bank.NewPrecompileTestSuite(integration.CreateEvmd)
	suite.Run(t, s)
}

func TestBankPrecompileIntegrationTestSuite(t *testing.T) {
	bank.TestIntegrationSuite(t, integration.CreateEvmd)
}
