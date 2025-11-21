package gov

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/gov"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestGovPrecompileTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.GovPrecompileApp](integration.CreateEvmd, "evm.GovPrecompileApp")
	s := gov.NewPrecompileTestSuite(create)
	suite.Run(t, s)
}

func TestGovPrecompileIntegrationTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.GovPrecompileApp](integration.CreateEvmd, "evm.GovPrecompileApp")
	gov.TestPrecompileIntegrationTestSuite(t, create)
}
