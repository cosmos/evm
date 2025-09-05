package erc20factory

import (
	"testing"

	"github.com/cosmos/evm/evmd/tests/integration"
	factory "github.com/cosmos/evm/tests/integration/precompiles/erc20factory"
	"github.com/stretchr/testify/suite"
)

func TestErc20FactoryPrecompileTestSuite(t *testing.T) {
	s := factory.NewPrecompileTestSuite(integration.CreateEvmd)
	suite.Run(t, s)
}
