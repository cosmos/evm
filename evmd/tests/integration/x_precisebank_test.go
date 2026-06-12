package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/tests/integration/x/precisebank"
)

func TestPreciseBankGenesis(t *testing.T) {
	s := precisebank.NewGenesisTestSuite(CreateEvmd)
	suite.Run(t, s)
}

func TestPreciseBankKeeper(t *testing.T) {
	s := precisebank.NewKeeperIntegrationTestSuite(CreateEvmd)
	suite.Run(t, s)
}
