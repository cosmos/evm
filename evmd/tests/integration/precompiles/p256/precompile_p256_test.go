package p256

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/p256"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestP256PrecompileTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.P256PrecompileApp](integration.CreateEvmd, "evm.P256PrecompileApp")
	s := p256.NewPrecompileTestSuite(create)
	suite.Run(t, s)
}

func TestP256PrecompileIntegrationTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.P256PrecompileApp](integration.CreateEvmd, "evm.P256PrecompileApp")
	p256.TestPrecompileIntegrationTestSuite(t, create)
}
