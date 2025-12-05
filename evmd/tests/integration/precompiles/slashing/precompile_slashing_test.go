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

func TestSlashingPrecompileTestSuiteWithBlockSTM(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.SlashingPrecompileApp](CreateEvmdWithBlockSTM, "evm.SlashingPrecompileApp")
	s := slashing.NewPrecompileTestSuite(create)
	suite.Run(t, s)
}

func TestSlashingPrecompileIntegrationTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.SlashingPrecompileApp](CreateEvmd, "evm.SlashingPrecompileApp")
	slashing.TestPrecompileIntegrationTestSuite(t, create)
}

func TestSlashingPrecompileIntegrationTestSuiteWithBlockSTM(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.SlashingPrecompileApp](CreateEvmdWithBlockSTM, "evm.SlashingPrecompileApp")
	slashing.TestPrecompileIntegrationTestSuite(t, create)
}
