package ics20

import (
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

//go:generate go run github.com/yihuang/go-abi/cmd -input abi.json -output ics20.abi.go -external-tuples PageRequest=cmn.PageRequest,PageResponse=cmn.PageResponse,Height=cmn.Height -imports cmn=github.com/cosmos/evm/precompiles/common

var _ vm.PrecompiledContract = &Precompile{}

type Precompile struct {
	cmn.Precompile

	bankKeeper     cmn.BankKeeper
	stakingKeeper  cmn.StakingKeeper
	transferKeeper cmn.TransferKeeper
	channelKeeper  cmn.ChannelKeeper
}

// NewPrecompile creates a new ICS-20 Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	bankKeeper cmn.BankKeeper,
	stakingKeeper cmn.StakingKeeper,
	transferKeeper cmn.TransferKeeper,
	channelKeeper cmn.ChannelKeeper,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:           storetypes.KVGasConfig(),
			TransientKVGasConfig:  storetypes.TransientGasConfig(),
			ContractAddress:       common.HexToAddress(evmtypes.ICS20PrecompileAddress),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		bankKeeper:     bankKeeper,
		transferKeeper: transferKeeper,
		channelKeeper:  channelKeeper,
		stakingKeeper:  stakingKeeper,
	}
}

// RequiredGas calculates the precompiled contract's base gas rate.
func (p Precompile) RequiredGas(input []byte) uint64 {
	// NOTE: This check avoid panicking when trying to decode the method ID
	if len(input) < 4 {
		return 0
	}

	methodID := binary.BigEndian.Uint32(input[:4])

	return p.Precompile.RequiredGas(input, p.IsTransaction(methodID))
}

func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, evm.StateDB, contract, readonly)
	})
}

func (p Precompile) Execute(ctx sdk.Context, stateDB vm.StateDB, contract *vm.Contract, readOnly bool) ([]byte, error) {
	methodID, input, err := cmn.ParseMethod(contract.Input, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	var bz []byte

	switch methodID {
	// ICS20 transactions
	case TransferID:
		bz, err = p.Transfer(ctx, contract, stateDB, input)
	// ICS20 queries
	case DenomID:
		bz, err = p.Denom(ctx, contract, input)
	case DenomsID:
		bz, err = p.Denoms(ctx, contract, input)
	case DenomHashID:
		bz, err = p.DenomHash(ctx, contract, input)
	default:
		return nil, fmt.Errorf("unknown method: %d", methodID)
	}

	return bz, err
}

// IsTransaction checks if the given method ID corresponds to a transaction or query.
//
// Available ics20 transactions are:
//   - Transfer
func (Precompile) IsTransaction(methodID uint32) bool {
	switch methodID {
	case TransferID:
		return true
	default:
		return false
	}
}
