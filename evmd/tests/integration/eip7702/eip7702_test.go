package eip7702

import (
	"testing"

	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/eip7702"
)

func TestEIP7702IntegrationTestSuite(t *testing.T) {
	eip7702.TestEIP7702IntegrationTestSuite(t, integration.CreateEvmd)
}
