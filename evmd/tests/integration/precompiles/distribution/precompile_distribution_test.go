package distribution

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/distribution"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestDistributionPrecompileTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.DistributionPrecompileApp](integration.CreateEvmd, "evm.DistributionPrecompileApp")
	s := distribution.NewPrecompileTestSuite(create)
	suite.Run(t, s)
}

func TestDistributionPrecompileIntegrationTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.DistributionPrecompileApp](integration.CreateEvmd, "evm.DistributionPrecompileApp")
	distribution.TestPrecompileIntegrationTestSuite(t, create)
}
