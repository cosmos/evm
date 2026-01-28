package common

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/common"
)

func TestStaticCallTestSuite(t *testing.T) {
	s := common.NewStaticCallTestSuite(integration.CreateEvmd)
	suite.Run(t, s)
}
