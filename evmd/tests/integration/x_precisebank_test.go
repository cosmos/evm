package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/x/precisebank"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestPreciseBankGenesis(t *testing.T) {
	s := precisebank.NewGenesisTestSuite(CreateEvmd)
	suite.Run(t, s)
}

func TestPreciseBankKeeper(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmd, "evm.IntegrationNetworkApp")
	s := precisebank.NewKeeperIntegrationTestSuite(create)
	suite.Run(t, s)
}
