package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/rpc/backend"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestBackend(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmd, "evm.IntegrationNetworkApp")
	s := backend.NewTestSuite(create)
	suite.Run(t, s)
}
