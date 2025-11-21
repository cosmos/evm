package distribution

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/distribution"
)

func TestDistributionPrecompileTestSuite(t *testing.T) {
	s := distribution.NewPrecompileTestSuite(integration.CreateEvmd)
	suite.Run(t, s)
}

func TestDistributionPrecompileIntegrationTestSuite(t *testing.T) {
	distribution.TestPrecompileIntegrationTestSuite(t, integration.CreateEvmd)
}
