package common

import (
	"errors"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func setInterpreterReturnData(evm *vm.EVM, data []byte) {
	if evm == nil || evm.Interpreter() == nil {
		return
	}
	evm.Interpreter().SetReturnData(data)
}

// RevertDataCarrier is an error that carries ABI-encoded revert data.
type RevertDataCarrier interface {
	error
	RevertData() []byte
}

// RevertWithData carries ABI-encoded custom error data.
type RevertWithData struct {
	data []byte
}

func (e *RevertWithData) Error() string {
	return vm.ErrExecutionReverted.Error()
}

func (e *RevertWithData) RevertData() []byte {
	return e.data
}

// NewRevertWithSolidityError packs args using the provided module ABI's error definition.
// It avoids hardcoding ABI information in Go by relying on the Solidity-generated ABI.
func NewRevertWithSolidityError(moduleABI abi.ABI, errorName string, args ...interface{}) error {
	customErr, ok := moduleABI.Errors[errorName]
	if !ok {
		reason := "unknown solidity custom error: " + errorName
		revertReasonBz, encErr := evmtypes.RevertReasonBytes(reason)
		if encErr != nil {
			return errors.New(reason)
		}
		return &RevertWithData{data: revertReasonBz}
	}

	data, err := customErr.Inputs.Pack(args...)
	if err != nil {
		return err
	}
	return &RevertWithData{data: append(customErr.ID[:4], data...)}
}

// ReturnRevertError maps precompile failures to vm.ErrExecutionReverted, sets interpreter
// return data when possible, and returns the same revert bytes as the call result (for opCall).
// See https://github.com/cosmos/evm/issues/223
func ReturnRevertError(evm *vm.EVM, err error) ([]byte, error) {
	var carrier RevertDataCarrier
	if errors.As(err, &carrier) {
		data := carrier.RevertData()
		setInterpreterReturnData(evm, data)
		return data, vm.ErrExecutionReverted
	}

	revertReasonBz, encErr := evmtypes.RevertReasonBytes(err.Error())
	if encErr != nil {
		return nil, vm.ErrExecutionReverted
	}
	setInterpreterReturnData(evm, revertReasonBz)

	return revertReasonBz, vm.ErrExecutionReverted
}
