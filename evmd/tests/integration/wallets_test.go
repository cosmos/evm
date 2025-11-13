package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/wallets"
	testapp "github.com/cosmos/evm/testutil/app"
)

var walletsAppCreator = testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmd, "evm.IntegrationNetworkApp")

func TestLedgerTestSuite(t *testing.T) {
	s := wallets.NewLedgerTestSuite(walletsAppCreator)
	suite.Run(t, s)
}
