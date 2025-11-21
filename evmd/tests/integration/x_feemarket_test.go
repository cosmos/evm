package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/x/feemarket"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestFeeMarketKeeperTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmd, "evm.IntegrationNetworkApp")
	s := feemarket.NewTestKeeperTestSuite(create)
	suite.Run(t, s)
}
