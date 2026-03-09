package keeper_test

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/mock"

	statedbmocks "github.com/cosmos/evm/x/vm/statedb/mocks"
)

func (suite *KeeperTestSuite) TestGetPrecompileRecipientCallHook() {
	staticPrecompileAddr := common.HexToAddress("0x0000000000000000000000000000000000000001")
	dynamicPrecompileAddr := common.HexToAddress("0x0000000000000000000000000000000000001234")
	nonPrecompileAddr := common.HexToAddress("0x0000000000000000000000000000000000009999")
	caller := common.HexToAddress("0x1111111111111111111111111111111111111111")

	// recipient in active precompiles
	suite.Run("recipient in active precompiles", func() {
		mockStateDB := statedbmocks.NewStateDB(suite.T())
		precompileMap := map[common.Address]vm.PrecompiledContract{
			staticPrecompileAddr: &mockPrecompiledContract{address: staticPrecompileAddr},
		}

		evm := vm.NewEVM(vm.BlockContext{BlockNumber: big.NewInt(1), Time: 1}, mockStateDB, params.TestChainConfig, vm.Config{})
		evm.WithPrecompiles(precompileMap)

		mockStateDB.On("AddAddressToAccessList", staticPrecompileAddr).Return().Once()

		hook := suite.vmKeeper.GetPrecompileRecipientCallHook(suite.ctx)
		err := hook(evm, caller, staticPrecompileAddr)
		suite.Require().NoError(err)
		mockStateDB.AssertExpectations(suite.T())
	})

	// recipient not in active precompiles but is a dynamic precompile
	suite.Run("dynamic precompile found", func() {
		mockStateDB := statedbmocks.NewStateDB(suite.T())
		precompileMap := map[common.Address]vm.PrecompiledContract{
			staticPrecompileAddr: &mockPrecompiledContract{address: staticPrecompileAddr},
		}

		evm := vm.NewEVM(vm.BlockContext{BlockNumber: big.NewInt(1), Time: 1}, mockStateDB, params.TestChainConfig, vm.Config{})
		evm.WithPrecompiles(precompileMap)

		mockPrecompile := &mockPrecompiledContract{address: dynamicPrecompileAddr}
		suite.erc20Keeper.On("GetERC20PrecompileInstance", mock.Anything, dynamicPrecompileAddr).
			Return(mockPrecompile, true, nil).Once()
		mockStateDB.On("AddAddressToAccessList", dynamicPrecompileAddr).Return().Once()

		hook := suite.vmKeeper.GetPrecompileRecipientCallHook(suite.ctx)
		err := hook(evm, caller, dynamicPrecompileAddr)
		suite.Require().NoError(err)
		mockStateDB.AssertExpectations(suite.T())
	})

	// recipient is neither static nor dynamic precompile
	suite.Run("non-precompile address", func() {
		mockStateDB := statedbmocks.NewStateDB(suite.T())
		precompileMap := map[common.Address]vm.PrecompiledContract{
			staticPrecompileAddr: &mockPrecompiledContract{address: staticPrecompileAddr},
		}

		evm := vm.NewEVM(vm.BlockContext{BlockNumber: big.NewInt(1), Time: 1}, mockStateDB, params.TestChainConfig, vm.Config{})
		evm.WithPrecompiles(precompileMap)

		suite.erc20Keeper.On("GetERC20PrecompileInstance", mock.Anything, nonPrecompileAddr).
			Return(nil, false, nil).Once()

		hook := suite.vmKeeper.GetPrecompileRecipientCallHook(suite.ctx)
		err := hook(evm, caller, nonPrecompileAddr)
		suite.Require().NoError(err)
		mockStateDB.AssertExpectations(suite.T())
	})

	// empty active precompiles but dynamic precompile exists
	suite.Run("empty active precompiles with dynamic precompile", func() {
		mockStateDB := statedbmocks.NewStateDB(suite.T())
		precompileMap := map[common.Address]vm.PrecompiledContract{}

		evm := vm.NewEVM(vm.BlockContext{BlockNumber: big.NewInt(1), Time: 1}, mockStateDB, params.TestChainConfig, vm.Config{})
		evm.WithPrecompiles(precompileMap)

		mockPrecompile := &mockPrecompiledContract{address: dynamicPrecompileAddr}
		suite.erc20Keeper.On("GetERC20PrecompileInstance", mock.Anything, dynamicPrecompileAddr).
			Return(mockPrecompile, true, nil).Once()
		mockStateDB.On("AddAddressToAccessList", dynamicPrecompileAddr).Return().Once()

		hook := suite.vmKeeper.GetPrecompileRecipientCallHook(suite.ctx)
		err := hook(evm, caller, dynamicPrecompileAddr)
		suite.Require().NoError(err)
		mockStateDB.AssertExpectations(suite.T())
	})
}

type mockPrecompiledContract struct {
	address common.Address
}

func (m *mockPrecompiledContract) Address() common.Address {
	return m.address
}

func (m *mockPrecompiledContract) RequiredGas(input []byte) uint64 {
	return 0
}

func (m *mockPrecompiledContract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return nil, nil
}
