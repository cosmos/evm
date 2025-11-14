package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/wallets"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestLedgerTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmd, "evm.IntegrationNetworkApp")
	s := wallets.NewLedgerTestSuite(create)
	suite.Run(t, s)
}
