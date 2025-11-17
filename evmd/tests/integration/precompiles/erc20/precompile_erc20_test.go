package erc20

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/erc20"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestErc20PrecompileTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.Erc20PrecompileApp](integration.CreateEvmd, "evm.Erc20PrecompileApp")
	s := erc20.NewPrecompileTestSuite(create)
	suite.Run(t, s)
}

func TestErc20IntegrationTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.Erc20PrecompileApp](integration.CreateEvmd, "evm.Erc20PrecompileApp")
	erc20.TestIntegrationTestSuite(t, create)
}
