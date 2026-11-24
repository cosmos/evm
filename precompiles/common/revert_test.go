package common_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/staking"
	statedbmocks "github.com/cosmos/evm/x/vm/statedb/mocks"
)

func TestReturnRevertError_WithCustomData(t *testing.T) {
	stateDB := statedbmocks.NewStateDB(t)
	evm := vm.NewEVM(vm.BlockContext{BlockNumber: big.NewInt(1), Time: 1}, stateDB, params.TestChainConfig, vm.Config{})

	customErr := cmn.NewRevertWithSolidityError(staking.ABI, "RequesterIsNotMsgSender", common.Address{0x1}, common.Address{0x2})
	ret, err := cmn.ReturnRevertError(evm, customErr)

	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Len(t, ret, 4+32+32)
	require.Equal(t, ret, evm.ReturnData())
}

func TestReturnRevertError_WithStringFallback(t *testing.T) {
	stateDB := statedbmocks.NewStateDB(t)
	evm := vm.NewEVM(vm.BlockContext{BlockNumber: big.NewInt(1), Time: 1}, stateDB, params.TestChainConfig, vm.Config{})

	ret, err := cmn.ReturnRevertError(evm, errors.New("fallback message"))

	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.NotEmpty(t, ret)
}

func TestReturnRevertError_WhenSolidityErrorPackFails_FallsBackToErrorStringRevert(t *testing.T) {
	stateDB := statedbmocks.NewStateDB(t)
	evm := vm.NewEVM(vm.BlockContext{BlockNumber: big.NewInt(1), Time: 1}, stateDB, params.TestChainConfig, vm.Config{})

	// RequesterIsNotMsgSender(address,address) expects 2 args; provide 1 wrong-typed arg to force ABI pack failure.
	customErr := cmn.NewRevertWithSolidityError(staking.ABI, "RequesterIsNotMsgSender", 123)
	ret, err := cmn.ReturnRevertError(evm, customErr)

	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.NotEmpty(t, ret)
	require.Equal(t, ret, evm.ReturnData())

	reason, unpackErr := abi.UnpackRevert(ret)
	require.NoError(t, unpackErr)
	require.Contains(t, reason, "failed to pack solidity custom error RequesterIsNotMsgSender")
}
