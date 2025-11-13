package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/rpc/backend"
	testapp "github.com/cosmos/evm/testutil/app"
)

var backendAppCreator = testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmd, "evm.IntegrationNetworkApp")

func TestBackend(t *testing.T) {
	s := backend.NewTestSuite(backendAppCreator)
	suite.Run(t, s)
}
