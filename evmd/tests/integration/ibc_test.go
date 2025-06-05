package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/tests/integration/x/ibc"
)

func TestIBCKeeperTestSuite(t *testing.T) {
	s := ibc.NewKeeperTestSuite(CreateEvmd)
	suite.Run(t, s)
}
