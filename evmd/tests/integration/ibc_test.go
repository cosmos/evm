package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/x/ibc"
	testapp "github.com/cosmos/evm/testutil/app"
)

var ibcAppCreator = testapp.ToEvmAppCreator[evm.IBCIntegrationApp](CreateEvmd, "evm.IBCIntegrationApp")

func TestIBCKeeperTestSuite(t *testing.T) {
	s := ibc.NewKeeperTestSuite(ibcAppCreator)
	suite.Run(t, s)
}
