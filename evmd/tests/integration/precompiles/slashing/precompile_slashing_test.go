package slashing

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/precompiles/slashing"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestSlashingPrecompileTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.SlashingPrecompileApp](CreateEvmd, "evm.SlashingPrecompileApp")
	s := slashing.NewPrecompileTestSuite(create)
	suite.Run(t, s)
}

func TestStakingPrecompileIntegrationTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.SlashingPrecompileApp](CreateEvmd, "evm.SlashingPrecompileApp")
	slashing.TestPrecompileIntegrationTestSuite(t, create)
}
