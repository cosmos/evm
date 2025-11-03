package bech32

import (
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

//go:generate go run ../cmd -input abi.json -output bech32.abi.go

var _ vm.PrecompiledContract = &Precompile{}

// Precompile defines the precompiled contract for Bech32 encoding.
type Precompile struct {
	baseGas uint64
}

// NewPrecompile creates a new bech32 Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(baseGas uint64) (*Precompile, error) {
	if baseGas == 0 {
		return nil, fmt.Errorf("baseGas cannot be zero")
	}

	return &Precompile{
		baseGas: baseGas,
	}, nil
}

// Address defines the address of the bech32 precompiled contract.
func (Precompile) Address() common.Address {
	return common.HexToAddress(evmtypes.Bech32PrecompileAddress)
}

// RequiredGas calculates the contract gas use.
func (p Precompile) RequiredGas(_ []byte) uint64 {
	return p.baseGas
}

// Run executes the precompiled contract bech32 methods defined in the ABI.
func (p Precompile) Run(_ *vm.EVM, contract *vm.Contract, _ bool) (bz []byte, err error) {
	// NOTE: This check avoid panicking when trying to decode the method ID
	if len(contract.Input) < 4 {
		return nil, vm.ErrExecutionReverted
	}

	methodID := binary.BigEndian.Uint32(contract.Input[:4])
	argsBz := contract.Input[4:]

	switch methodID {
	case HexToBech32ID:
		var hexToBech32Args HexToBech32Call
		if _, err := hexToBech32Args.Decode(argsBz); err != nil {
			return nil, err
		}
		result, err := p.HexToBech32(hexToBech32Args)
		if err != nil {
			return nil, err
		}
		return result.Encode()
	case Bech32ToHexID:
		var bech32ToHexArgs Bech32ToHexCall
		if _, err := bech32ToHexArgs.Decode(argsBz); err != nil {
			return nil, err
		}
		result, err := p.Bech32ToHex(bech32ToHexArgs)
		if err != nil {
			return nil, err
		}
		return result.Encode()
	default:
		return nil, vm.ErrExecutionReverted
	}
}
