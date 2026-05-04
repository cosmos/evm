// Package debug defines test utilities that are meant for debugging the chain, and *not* for use in production.
package debug

import (
	"cosmossdk.io/errors"
	"fmt"
	errors2 "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/ethereum/go-ethereum/core/tracing"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/vm"

	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	cmn "github.com/cosmos/evm/precompiles/common"
)

// Precompile defines a debugging precompile for use in testing.
type Precompile struct {
	cmn.Precompile

	evmKeeper EVMKeeper
}

const DebugPrecompileAddress = "0x0000000000000000000000000000000000000799"

func NewPrecompile(bankKeeper cmn.BankKeeper, evmKeeper EVMKeeper) *Precompile {
	p := &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:           storetypes.KVGasConfig(),
			TransientKVGasConfig:  storetypes.TransientGasConfig(),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		evmKeeper: evmKeeper,
	}
	// SetAddress defines the address of the distribution compile contract.
	p.SetAddress(common.HexToAddress(DebugPrecompileAddress))
	return p
}

func (p Precompile) RequiredGas(input []byte) uint64 {
	return 1000
}

func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	stateDB, ok := evm.StateDB.(*statedb.StateDB)
	if !ok {
		return nil, errors.Wrap(errors2.ErrUnauthorized, "could not create statedb in debug precompile")
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
	err = stateDB.AddPrecompileFn(p.Address(), snapshot, events)
	if err != nil {
		return nil, err
	}

	// commit the current changes in the cache ctx
	// to get the updated state for the precompile call
	if err := stateDB.CommitWithCacheCtx(); err != nil {
		return nil, err
	}

	// Start the balance change handler before executing the precompile.
	var balanceHandler *cmn.BalanceHandler
	if p.BalanceHandlerFactory != nil {
		balanceHandler = p.BalanceHandlerFactory.NewBalanceHandler()
	}

	if balanceHandler != nil {
		balanceHandler.BeforeBalanceChange(ctx)
	}

	initialGas := ctx.GasMeter().GasConsumed()

	// set the default SDK gas configuration to track gas usage
	// we are changing the gas meter type, so it panics gracefully when out of gas
	ctx = ctx.WithGasMeter(storetypes.NewGasMeter(contract.Gas)).
		WithKVGasConfig(p.KvGasConfig).
		WithTransientKVGasConfig(p.TransientKVGasConfig)
	// we need to consume the gas that was already used by the EVM
	ctx.GasMeter().ConsumeGas(initialGas, "creating a new gas meter")

	// This handles any out of gas errors that may occur during the execution of a precompile tx or query.
	// It avoids panics and returns the out of gas error so the EVM can continue gracefully.
	defer cmn.HandleGasError(ctx, contract, initialGas, &err)()

	res, err := p.Execute(ctx, stateDB, contract, readonly)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	cost := ctx.GasMeter().GasConsumed() - initialGas

	if !contract.UseGas(cost, nil, tracing.GasChangeCallPrecompiledContract) {
		return nil, vm.ErrOutOfGas
	}

	// Process the native balance changes after the method execution.
	if balanceHandler != nil {
		if err := balanceHandler.AfterBalanceChange(ctx, stateDB); err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (p Precompile) Execute(ctx sdk.Context, stateDB vm.StateDB, contract *vm.Contract, readOnly bool) ([]byte, error) {
	switch contract.Input[0] {
	case 0: // callback()
		return p.Call0(ctx, stateDB, contract, readOnly)
	case 1: // call1()
		return p.Call1(ctx, stateDB, contract, readOnly)
	}
	return nil, fmt.Errorf("unknown method: %x", contract.Input[0])
}

func (p Precompile) Call0(ctx sdk.Context, stateDB vm.StateDB, contract *vm.Contract, readOnly bool) ([]byte, error) {
	// data := crypto.Keccak256([]byte("function callback()"))[:4]
	counter := new(big.Int).SetBytes(contract.Input[1:])
	counter = new(big.Int).Add(counter, big.NewInt(1))

	args := math.U256Bytes(counter)
	selector := []byte{0xff, 0x58, 0x5c, 0xaf}
	data := append(selector, args...)

	caller := contract.Caller()
	fmt.Printf("Execute debug precompile %s, %p\n", caller.String(), p.BalanceHandlerFactory)
	rsp, err := p.evmKeeper.CallEVMWithData(ctx, p.Address(), &caller, data, true)
	fmt.Println("callback response:", rsp.Ret, err)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (p Precompile) Call1(ctx sdk.Context, stateDB vm.StateDB, contract *vm.Contract, readOnly bool) ([]byte, error) {
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"debug_precompile",
			sdk.NewAttribute("address", p.Address().String()),
		),
	)
	return nil, nil
}
