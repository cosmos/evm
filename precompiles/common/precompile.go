package common

import (
	"errors"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/evm/x/vm/statedb"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NativeAction abstract the native execution logic of the stateful precompile, it's passed to the base `Precompile`
// struct, base `Precompile` struct will handle things the native context setup, gas management, panic recovery etc,
// before and after the execution.
//
// It's usually implemented by the precompile itself.
type NativeAction func(ctx sdk.Context) ([]byte, error)

// Precompile is the base struct for precompiles that requires to access cosmos native storage.
type Precompile struct {
	KvGasConfig          storetypes.GasConfig
	TransientKVGasConfig storetypes.GasConfig
	ContractAddress      common.Address

	// BalanceHandlerFactory is optional
	BalanceHandlerFactory *BalanceHandlerFactory
}

// RequiredGas calculates the base minimum required gas for a transaction or a query.
// It uses the method ID to determine if the input is a transaction or a query and
// uses the Cosmos SDK gas config flat cost and the flat per byte cost * len(argBz) to calculate the gas.
func (p Precompile) RequiredGas(input []byte, isTransaction bool) uint64 {
	if isTransaction {
		return p.KvGasConfig.WriteCostFlat + (p.KvGasConfig.WriteCostPerByte * uint64(len(input)))
	}

	return p.KvGasConfig.ReadCostFlat + (p.KvGasConfig.ReadCostPerByte * uint64(len(input)))
}

// Run prepare the native context to execute native action for stateful precompile,
// it manages the snapshot and revert of the multi-store.
func (p Precompile) RunNativeAction(evm *vm.EVM, contract *vm.Contract, action NativeAction) ([]byte, error) {
	bz, err := p.runNativeAction(evm, contract, action)
	if err != nil {
		return ReturnRevertError(evm, err)
	}

	return bz, nil
}

func (p Precompile) runNativeAction(evm *vm.EVM, contract *vm.Contract, action NativeAction) (bz []byte, err error) {
	stateDB, ok := evm.StateDB.(*statedb.StateDB)
	if !ok {
		return nil, errors.New(ErrNotRunInEvm)
	}

	// get the stateDB cache ctx
	ctx, err := stateDB.GetCacheContext()
	if err != nil {
		return nil, err
	}

	// take a snapshot of the current state before any changes
	// to be able to revert the changes
	snapshot := stateDB.MultiStoreSnapshot()
	events := ctx.EventManager().Events()

	// add precompileCall entry on the stateDB journal
	// this allows to revert the changes within an evm tx
	if err := stateDB.AddPrecompileFn(snapshot, events); err != nil {
		return nil, err
	}

	// commit the current changes in the cache ctx
	// to get the updated state for the precompile call
	if err := stateDB.CommitWithCacheCtx(); err != nil {
		return nil, err
	}

	initialGas := ctx.GasMeter().GasConsumed()

	defer HandleGasError(ctx, contract, initialGas, &err)()

	// set the default SDK gas configuration to track gas usage
	// we are changing the gas meter type, so it panics gracefully when out of gas
	ctx = ctx.WithGasMeter(storetypes.NewGasMeter(contract.Gas)).
		WithKVGasConfig(p.KvGasConfig).
		WithTransientKVGasConfig(p.TransientKVGasConfig)

	// we need to consume the gas that was already used by the EVM
	ctx.GasMeter().ConsumeGas(initialGas, "creating a new gas meter")

	var balanceHandler *BalanceHandler
	if p.BalanceHandlerFactory != nil {
		balanceHandler = p.BalanceHandlerFactory.NewBalanceHandler()
	}

	if balanceHandler != nil {
		balanceHandler.BeforeBalanceChange(ctx)
	}

	bz, err = action(ctx)
	if err != nil {
		return bz, err
	}

	cost := ctx.GasMeter().GasConsumed() - initialGas

	if !contract.UseGas(cost, nil, tracing.GasChangeCallPrecompiledContract) {
		return nil, vm.ErrOutOfGas
	}

	if balanceHandler != nil {
		if err := balanceHandler.AfterBalanceChange(ctx, stateDB); err != nil {
			return nil, err
		}
	}

	return bz, nil
}

// SetupABI runs the initial setup required to run a transaction or a query.
// It returns the ABI method, initial gas and calling arguments.
func SetupABI(
	api abi.ABI,
	contract *vm.Contract,
	readOnly bool,
	isTransaction func(name *abi.Method) bool,
) (method *abi.Method, args []interface{}, err error) {
	// NOTE: This is a special case where the calling transaction does not specify a function name.
	// In this case we default to a `fallback` or `receive` function on the contract.

	// Simplify the calldata checks
	isEmptyCallData := len(contract.Input) == 0
	isShortCallData := len(contract.Input) > 0 && len(contract.Input) < 4
	isStandardCallData := len(contract.Input) >= 4

	switch {
	// Case 1: Calldata is empty
	case isEmptyCallData:
		method, err = emptyCallData(api, contract)

	// Case 2: calldata is non-empty but less than 4 bytes needed for a method
	case isShortCallData:
		method, err = methodIDCallData(api)

	// Case 3: calldata is non-empty and contains the minimum 4 bytes needed for a method
	case isStandardCallData:
		method, err = standardCallData(api, contract)
	}

	if err != nil {
		return nil, nil, err
	}

	// return error if trying to write to state during a read-only call
	if readOnly && isTransaction(method) {
		return nil, nil, vm.ErrWriteProtection
	}

	// if the method type is `function` continue looking for arguments
	if method.Type == abi.Function {
		argsBz := contract.Input[4:]
		args, err = method.Inputs.Unpack(argsBz)
		if err != nil {
			return nil, nil, err
		}
	}

	return method, args, nil
}

// HandleGasError handles the out of gas panic by resetting the gas meter and returning an error.
// This is used in order to avoid panics and to allow for the EVM to continue cleanup if the tx or query run out of gas.
func HandleGasError(ctx sdk.Context, contract *vm.Contract, initialGas storetypes.Gas, err *error) func() {
	return func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case storetypes.ErrorOutOfGas:
				// update contract gas
				usedGas := ctx.GasMeter().GasConsumed() - initialGas
				_ = contract.UseGas(usedGas, nil, tracing.GasChangeCallFailedExecution)

				*err = vm.ErrOutOfGas
				// FIXME: add InfiniteGasMeter with previous Gas limit.
				ctx = ctx.WithKVGasConfig(storetypes.GasConfig{}).
					WithTransientKVGasConfig(storetypes.GasConfig{})
			default:
				panic(r)
			}
		}
	}
}

func (p Precompile) Address() common.Address {
	return p.ContractAddress
}

func (p *Precompile) SetAddress(addr common.Address) {
	p.ContractAddress = addr
}

// emptyCallData is a helper function that returns the method to be called when the calldata is empty.
func emptyCallData(api abi.ABI, contract *vm.Contract) (method *abi.Method, err error) {
	switch {
	// Case 1.1: Send call or transfer tx - 'receive' is called if present and value is transferred
	case contract.Value().Sign() > 0 && api.HasReceive():
		return &api.Receive, nil
	// Case 1.2: Either 'receive' is not present, or no value is transferred - call 'fallback' if present
	case api.HasFallback():
		return &api.Fallback, nil
	// Case 1.3: Neither 'receive' nor 'fallback' are present - return error
	default:
		return nil, vm.ErrExecutionReverted
	}
}

// methodIDCallData is a helper function that returns the method to be called when the calldata is less than 4 bytes.
func methodIDCallData(api abi.ABI) (method *abi.Method, err error) {
	// Case 2.2: calldata contains less than 4 bytes needed for a method and 'fallback' is not present - return error
	if !api.HasFallback() {
		return nil, vm.ErrExecutionReverted
	}
	// Case 2.1: calldata contains less than 4 bytes needed for a method - 'fallback' is called if present
	return &api.Fallback, nil
}

// standardCallData is a helper function that returns the method to be called when the calldata is 4 bytes or more.
func standardCallData(api abi.ABI, contract *vm.Contract) (method *abi.Method, err error) {
	methodID := contract.Input[:4]
	// NOTE: this function iterates over the method map and returns
	// the method with the given ID
	method, err = api.MethodById(methodID)

	// Case 3.1 calldata contains a non-existing method ID, and `fallback` is not present - return error
	if err != nil && !api.HasFallback() {
		return nil, err
	}

	// Case 3.2: calldata contains a non-existing method ID - 'fallback' is called if present
	if err != nil && api.HasFallback() {
		return &api.Fallback, nil
	}

	return method, nil
}
