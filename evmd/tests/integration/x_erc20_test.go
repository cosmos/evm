package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/x/erc20"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestERC20GenesisTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.Erc20IntegrationApp](CreateEvmd, "evm.Erc20IntegrationApp")
	suite.Run(t, erc20.NewGenesisTestSuite(create))
}

func TestERC20KeeperTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.Erc20IntegrationApp](CreateEvmd, "evm.Erc20IntegrationApp")
	s := erc20.NewKeeperTestSuite(create)
	suite.Run(t, s)
}

func TestERC20PrecompileIntegrationTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.Erc20IntegrationApp](CreateEvmd, "evm.Erc20IntegrationApp")
	erc20.TestPrecompileIntegrationTestSuite(t, create)
}
