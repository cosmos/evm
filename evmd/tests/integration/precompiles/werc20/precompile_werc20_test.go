package werc20

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/werc20"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestWERC20PrecompileUnitTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.WERCP20PrecompileApp](integration.CreateEvmd, "evm.WERCP20PrecompileApp")
	s := werc20.NewPrecompileUnitTestSuite(create)
	suite.Run(t, s)
}

func TestWERC20PrecompileIntegrationTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.WERCP20PrecompileApp](integration.CreateEvmd, "evm.WERCP20PrecompileApp")
	werc20.TestPrecompileIntegrationTestSuite(t, create)
}
