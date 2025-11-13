package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/x/ibc/callbacks"
	testapp "github.com/cosmos/evm/testutil/app"
)

var ibcCallbackAppCreator = testapp.ToEvmAppCreator[evm.IBCCallbackIntegrationApp](CreateEvmd, "evm.IBCCallbackIntegrationApp")

func TestIBCCallback(t *testing.T) {
	suite.Run(t, callbacks.NewKeeperTestSuite(ibcCallbackAppCreator))
}
