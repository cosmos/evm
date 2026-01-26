package vm

import (
	"testing"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/x/vm"
	testapp "github.com/cosmos/evm/testutil/app"

	"github.com/stretchr/testify/suite"
)

func TestKeeperTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.VMIntegrationApp](integration.CreateEvmd, "evm.VMIntegrationApp")
	s := vm.NewKeeperTestSuite(create)
	s.EnableFeemarket = false
	s.EnableLondonHF = true
	suite.Run(t, s)
}

func TestNestedEVMExtensionCallSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.VMIntegrationApp](integration.CreateEvmd, "evm.VMIntegrationApp")
	s := vm.NewNestedEVMExtensionCallSuite(create)
	suite.Run(t, s)
}

func TestGenesisTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.VMIntegrationApp](integration.CreateEvmd, "evm.VMIntegrationApp")
	s := vm.NewGenesisTestSuite(create)
	suite.Run(t, s)
}

func TestVmAnteTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.VMIntegrationApp](integration.CreateEvmd, "evm.VMIntegrationApp")
	s := vm.NewEvmAnteTestSuite(create)
	suite.Run(t, s)
}

func TestIterateContracts(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.VMIntegrationApp](integration.CreateEvmd, "evm.VMIntegrationApp")
	vm.TestIterateContracts(t, create)
}
