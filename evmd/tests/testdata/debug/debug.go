// Package debug defines test utilities that are meant for debugging the chain, and *not* for use in production.
package debug

import (
	"fmt"
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
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:           storetypes.KVGasConfig(),
			TransientKVGasConfig:  storetypes.TransientGasConfig(),
			ContractAddress:       common.HexToAddress(DebugPrecompileAddress),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		evmKeeper: evmKeeper,
	}
}

func (p Precompile) RequiredGas(input []byte) uint64 {
	return 1000
}

func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, evm.StateDB, contract, readonly)
	})
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
	rsp, err := p.evmKeeper.CallEVMWithData(ctx, p.Address(), &caller, data, true, nil)
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
