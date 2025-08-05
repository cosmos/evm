package werc20

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/werc20"
)

func TestWERC20PrecompileUnitTestSuite(t *testing.T) {
	s := werc20.NewPrecompileUnitTestSuite(integration.CreateEvmd)
	suite.Run(t, s)
}

func TestWERC20PrecompileIntegrationTestSuite(t *testing.T) {
	werc20.TestPrecompileIntegrationTestSuite(t, integration.CreateEvmd)
}
