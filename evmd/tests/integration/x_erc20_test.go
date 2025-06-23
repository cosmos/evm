package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/tests/integration/x/erc20"
)

func TestERC20GenesisTestSuite(t *testing.T) {
	suite.Run(t, erc20.NewGenesisTestSuite(CreateEvmd))
}

func TestERC20KeeperTestSuite(t *testing.T) {
	s := erc20.NewKeeperTestSuite(CreateEvmd)
	suite.Run(t, s)
}

func TestERC20PrecompileIntegrationTestSuite(t *testing.T) {
	erc20.TestPrecompileIntegrationTestSuite(t, CreateEvmd)
}
