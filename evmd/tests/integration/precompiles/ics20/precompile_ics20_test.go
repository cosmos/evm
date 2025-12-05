package ics20

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/ibc"
	"github.com/cosmos/evm/tests/integration/precompiles/ics20"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestICS20PrecompileTestSuite(t *testing.T) {
	create := testapp.ToIBCAppCreator[evm.ICS20PrecompileApp](ibc.SetupEvmd, "evm.ICS20PrecompileApp")
	s := ics20.NewPrecompileTestSuite(t, create)
	suite.Run(t, s)
}

func TestICS20PrecompileTestSuiteWithBlockSTM(t *testing.T) {
	create := testapp.ToIBCAppCreator[evm.ICS20PrecompileApp](ibc.SetupEvmdWithBlockSTM, "evm.ICS20PrecompileApp")
	s := ics20.NewPrecompileTestSuite(t, create)
	suite.Run(t, s)
}

func TestICS20PrecompileIntegrationTestSuite(t *testing.T) {
	create := testapp.ToIBCAppCreator[evm.ICS20PrecompileApp](ibc.SetupEvmd, "evm.ICS20PrecompileApp")
	ics20.TestPrecompileIntegrationTestSuite(t, create)
}

func TestICS20PrecompileIntegrationTestSuiteWithBlockSTM(t *testing.T) {
	create := testapp.ToIBCAppCreator[evm.ICS20PrecompileApp](ibc.SetupEvmdWithBlockSTM, "evm.ICS20PrecompileApp")
	ics20.TestPrecompileIntegrationTestSuite(t, create)
}
