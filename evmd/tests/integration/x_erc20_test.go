package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/x/erc20"
	testapp "github.com/cosmos/evm/testutil/app"
)

var erc20AppCreator = testapp.ToEvmAppCreator[evm.Erc20IntegrationApp](CreateEvmd, "evm.Erc20IntegrationApp")

func TestERC20GenesisTestSuite(t *testing.T) {
	suite.Run(t, erc20.NewGenesisTestSuite(erc20AppCreator))
}

func TestERC20KeeperTestSuite(t *testing.T) {
	s := erc20.NewKeeperTestSuite(erc20AppCreator)
	suite.Run(t, s)
}

func TestERC20PrecompileIntegrationTestSuite(t *testing.T) {
	erc20.TestPrecompileIntegrationTestSuite(t, erc20AppCreator)
}
