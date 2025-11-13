package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/eip712"
	testapp "github.com/cosmos/evm/testutil/app"
)

var eip712AppCreator = testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmd, "evm.IntegrationNetworkApp")

func TestEIP712TestSuite(t *testing.T) {
	s := eip712.NewTestSuite(eip712AppCreator, false)
	suite.Run(t, s)

	// Note that we don't test the Legacy EIP-712 Extension, since that case
	// is sufficiently covered by the AnteHandler tests.
	s = eip712.NewTestSuite(eip712AppCreator, true)
	suite.Run(t, s)
}
