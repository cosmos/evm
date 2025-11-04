package ics02

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	ibcutils "github.com/cosmos/evm/ibc"
	cmn "github.com/cosmos/evm/precompiles/common"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

//go:generate go run ../cmd -input abi.json -output ics02.abi.go

var _ vm.PrecompiledContract = (*Precompile)(nil)

// Precompile defines the precompiled contract for ICS02.
type Precompile struct {
	cmn.Precompile

	cdc          codec.Codec
	clientKeeper ibcutils.ClientKeeper
}

// NewPrecompile creates a new Client Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	cdc codec.Codec,
	clientKeeper ibcutils.ClientKeeper,
) *Precompile {
	return &Precompile{
		cdc: cdc,
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.GasConfig{},
			TransientKVGasConfig: storetypes.GasConfig{},
			ContractAddress:      common.HexToAddress(evmtypes.ICS02PrecompileAddress),
		},
		clientKeeper: clientKeeper,
	}
}

// RequiredGas calculates the precompiled contract's base gas rate.
func (p Precompile) RequiredGas(input []byte) uint64 {
	methodID, input, err := cmn.SplitMethodID(input)
	if err != nil {
		return 0
	}

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
	input = input[4:] // remove method ID

	switch methodID {
	case UpdateClientID:
		return cmn.Run(ctx, p.UpdateClient, input)
	case VerifyMembershipID:
		return cmn.Run(ctx, p.VerifyMembership, input)
	case VerifyNonMembershipID:
		return cmn.Run(ctx, p.VerifyNonMembership, input)
	// queries:
	case GetClientStateID:
		return cmn.Run(ctx, p.GetClientState, input)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, methodID)
	}
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
func (Precompile) IsTransaction(method uint32) bool {
	switch method {
	case UpdateClientID,
		VerifyMembershipID,
		VerifyNonMembershipID:
		return true
	default:
		// GetClientStateMethod is the only query method.
		return false
	}
}

// Logger returns a precompile-specific logger.
func (p Precompile) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("evm extension", "ics02")
}
