package eip7702

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/eip7702"
)

func TestEIP7702IntegrationTestSuite(t *testing.T) {
	suite.Run(t, eip7702.NewEIP7702IntegrationTestSuite(integration.CreateEvmd))
}
