package erc20

import (
	"testing"

	"cosmosevm.io/evmd/tests/integration"
	"github.com/stretchr/testify/suite"

	erc21 "github.com/cosmos/evm/tests/integration/precompiles/erc20"
)

func TestErc20PrecompileTestSuite(t *testing.T) {
	s := erc21.NewPrecompileTestSuite(integration.CreateEvmd)
	suite.Run(t, s)
}

func TestErc20IntegrationTestSuite(t *testing.T) {
	erc21.TestIntegrationTestSuite(t, integration.CreateEvmd)
}
