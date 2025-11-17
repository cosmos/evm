package bech32

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/bech32"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestBech32PrecompileTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.Bech32PrecompileApp](integration.CreateEvmd, "evm.Bech32PrecompileApp")
	s := bech32.NewPrecompileTestSuite(create)
	suite.Run(t, s)
}
