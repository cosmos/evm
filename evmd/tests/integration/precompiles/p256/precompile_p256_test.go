package p256

import (
	"testing"

	"cosmosevm.io/evmd/tests/integration"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/tests/integration/precompiles/p256"
)

func TestP256PrecompileTestSuite(t *testing.T) {
	s := p256.NewPrecompileTestSuite(integration.CreateEvmd)
	suite.Run(t, s)
}

func TestP256PrecompileIntegrationTestSuite(t *testing.T) {
	p256.TestPrecompileIntegrationTestSuite(t, integration.CreateEvmd)
}
