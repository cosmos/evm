package slashing

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	_ "embed"

	cmn "github.com/cosmos/evm/precompiles/common"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/core/address"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
)

//go:generate go run github.com/yihuang/go-abi/cmd -input abi.json -output slashing.abi.go -package slashing -external-tuples Dec=cmn.Dec,PageRequest=cmn.PageRequest -imports cmn=github.com/cosmos/evm/precompiles/common

var _ vm.PrecompiledContract = &Precompile{}

// Precompile defines the precompiled contract for slashing.
type Precompile struct {
	cmn.Precompile

	slashingKeeper    cmn.SlashingKeeper
	slashingMsgServer slashingtypes.MsgServer
	consCodec         runtime.ConsensusAddressCodec
	valCodec          runtime.ValidatorAddressCodec
}

// NewPrecompile creates a new slashing Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	slashingKeeper cmn.SlashingKeeper,
	slashingMsgServer slashingtypes.MsgServer,
	bankKeeper cmn.BankKeeper,
	valCdc, consCdc address.Codec,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:           storetypes.KVGasConfig(),
			TransientKVGasConfig:  storetypes.TransientGasConfig(),
			ContractAddress:       common.HexToAddress(evmtypes.SlashingPrecompileAddress),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		slashingKeeper:    slashingKeeper,
		slashingMsgServer: slashingMsgServer,
		valCodec:          valCdc,
		consCodec:         consCdc,
	}
}

// RequiredGas returns the required bare minimum gas to execute the precompile.
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

	switch methodID {
	// Slashing transactions
	case UnjailID:
		return cmn.RunWithStateDB(ctx, p.Unjail, input, stateDB, contract)
	// Slashing queries
	case GetParamsID:
		return cmn.Run(ctx, p.GetParams, input)
	case GetSigningInfoID:
		return cmn.Run(ctx, p.GetSigningInfo, input)
	case GetSigningInfosID:
		return cmn.Run(ctx, p.GetSigningInfos, input)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, methodID)
	}
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
//
// Available slashing transactions are:
// - Unjail
func (Precompile) IsTransaction(method uint32) bool {
	switch method {
	case UnjailID:
		return true
	default:
		return false
	}
}

// Logger returns a precompile-specific logger.
func (p Precompile) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("evm extension", "slashing")
}
