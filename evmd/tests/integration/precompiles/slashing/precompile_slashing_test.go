package slashing

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/slashing"
	testapp "github.com/cosmos/evm/testutil/app"
)

var evmAppCreator = testapp.ToEvmAppCreator[evm.SlashingPrecompileApp](integration.CreateEvmd, "evm.SlashingPrecompileApp")

func TestSlashingPrecompileTestSuite(t *testing.T) {
	s := slashing.NewPrecompileTestSuite(evmAppCreator)
	suite.Run(t, s)
}

func TestStakingPrecompileIntegrationTestSuite(t *testing.T) {
	slashing.TestPrecompileIntegrationTestSuite(t, evmAppCreator)
}
