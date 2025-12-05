package precisebank

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/x/precisebank"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestPreciseBankGenesis(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmd, "evm.IntegrationNetworkApp")
	s := precisebank.NewGenesisTestSuite(create)
	suite.Run(t, s)
}

func TestPreciseBankGenesisWithBlockSTM(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmd, "evm.IntegrationNetworkApp")
	s := precisebank.NewGenesisTestSuite(create)
	suite.Run(t, s)
}

func TestPreciseBankKeeper(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmd, "evm.IntegrationNetworkApp")
	s := precisebank.NewKeeperIntegrationTestSuite(create)
	suite.Run(t, s)
}

// TODO: enable this test after fix block-stm related interface implementation of x/precisebank module
// func TestPreciseBankKeeperWithBlockSTM(t *testing.T) {
// 	create := testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmdWithBlockSTM, "evm.IntegrationNetworkApp")
// 	s := precisebank.NewKeeperIntegrationTestSuite(create)
// 	suite.Run(t, s)
// }
