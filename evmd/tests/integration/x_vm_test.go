package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/tests/integration/x/vm"
)

func TestKeeperTestSuite(t *testing.T) {
	s := vm.NewKeeperTestSuite(CreateEvmd)
	s.EnableFeemarket = false
	s.EnableLondonHF = true
	suite.Run(t, s)
}

func TestNestedEVMExtensionCallSuite(t *testing.T) {
	s := vm.NewNestedEVMExtensionCallSuite(CreateEvmd)
	suite.Run(t, s)
}

func TestGenesisTestSuite(t *testing.T) {
	s := vm.NewGenesisTestSuite(CreateEvmd)
	suite.Run(t, s)
}

func TestVmAnteTestSuite(t *testing.T) {
	s := vm.NewEvmAnteTestSuite(CreateEvmd)
	suite.Run(t, s)
}

func TestIterateContracts(t *testing.T) {
	vm.TestIterateContracts(t, CreateEvmd)
}
