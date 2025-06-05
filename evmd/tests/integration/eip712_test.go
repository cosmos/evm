package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/tests/integration/eip712"
)

func TestEIP712TestSuite(t *testing.T) {
	s := eip712.NewTestSuite(CreateEvmd, false)
	suite.Run(t, s)

	// Note that we don't test the Legacy EIP-712 Extension, since that case
	// is sufficiently covered by the AnteHandler tests.
	s = eip712.NewTestSuite(CreateEvmd, true)
	suite.Run(t, s)
}
