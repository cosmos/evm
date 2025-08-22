package bech32

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/bech32"
)

func TestBech32PrecompileTestSuite(t *testing.T) {
	s := bech32.NewPrecompileTestSuite(integration.CreateEvmd)
	suite.Run(t, s)
}
