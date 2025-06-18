package erc20factory

import (
	"testing"

	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/erc20factory"
	"github.com/stretchr/testify/suite"
)

func TestERC20FactoryPrecompileTestSuite(t *testing.T) {
	s := erc20factory.NewPrecompileTestSuite(integration.CreateEvmd)
	suite.Run(t, s)
}
