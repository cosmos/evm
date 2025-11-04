package common

import (
	"encoding/binary"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/yihuang/go-abi"

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

// SplitMethodID splits the method id from the input data.
func SplitMethodID(input []byte) (uint32, []byte, error) {
	if len(input) < 4 {
		return 0, nil, errors.New("invalid input length")
	}

	methodID := binary.BigEndian.Uint32(input)
	return methodID, input[4:], nil
}

// ParseMethod splits method id, and check if it's allowed in readOnly mode.
func ParseMethod(input []byte, readOnly bool, isTransaction func(uint32) bool) (uint32, []byte, error) {
	methodID, input, err := SplitMethodID(input)
	if err != nil {
		return 0, nil, err
	}

	if readOnly && isTransaction(methodID) {
		return 0, nil, vm.ErrWriteProtection
	}

	return methodID, input[4:], nil
}

func Run[I any, PI interface {
	*I
	abi.Decode
}, O abi.Encode](
	ctx sdk.Context,
	fn func(sdk.Context, I) (O, error),
	input []byte,
) ([]byte, error) {
	var in I
	if _, err := PI(&in).Decode(input); err != nil {
		return nil, err
	}

	out, err := fn(ctx, in)
	if err != nil {
		return nil, err
	}

	return out.Encode()
}

func RunWithStateDB[I any, PI interface {
	*I
	abi.Decode
}, O abi.Encode](
	ctx sdk.Context,
	fn func(sdk.Context, I, vm.StateDB, *vm.Contract) (O, error),
	input []byte,
	stateDB vm.StateDB,
	contract *vm.Contract,
) ([]byte, error) {
	var in I
	if _, err := PI(&in).Decode(input); err != nil {
		return nil, err
	}

	out, err := fn(ctx, in, stateDB, contract)
	if err != nil {
		return nil, err
	}

	return out.Encode()
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
