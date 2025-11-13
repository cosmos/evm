package mempool

import (
	"testing"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	testapp "github.com/cosmos/evm/testutil/app"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/tests/integration/mempool"
)

func TestMempoolIntegrationTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](integration.CreateEvmd, "evm.IntegrationNetworkApp")
	suite.Run(t, mempool.NewMempoolIntegrationTestSuite(create))
}
