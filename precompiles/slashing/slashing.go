package slashing

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
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

var _ vm.PrecompiledContract = &Precompile{}

var (
	// Embed abi json file to the executable binary. Needed when importing as dependency.
	//
	//go:embed abi.json
	f   []byte
	ABI abi.ABI
)

func init() {
	var err error
	ABI, err = abi.JSON(bytes.NewReader(f))
	if err != nil {
		panic(err)
	}
}

// Precompile defines the precompiled contract for slashing.
type Precompile struct {
	cmn.Precompile

	abi.ABI
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
		ABI:               ABI,
		slashingKeeper:    slashingKeeper,
		slashingMsgServer: slashingMsgServer,
		valCodec:          valCdc,
		consCodec:         consCdc,
	}
}

// RequiredGas calculates the precompiled contract's base gas rate.
func (p Precompile) RequiredGas(input []byte) uint64 {
	// NOTE: This check avoid panicking when trying to decode the method ID
	if len(input) < 4 {
		return 0
	}
	methodID := input[:4]

	method, err := p.MethodById(methodID)
	if err != nil {
		// This should never happen since this method is going to fail during Run
		return 0
	}

	return p.Precompile.RequiredGas(input, p.IsTransaction(method))
}

func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, evm.StateDB, contract, readonly)
	})
}

func (p Precompile) Execute(ctx sdk.Context, stateDB vm.StateDB, contract *vm.Contract, readOnly bool) ([]byte, error) {
	method, args, err := cmn.SetupABI(p.ABI, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	var bz []byte

	switch method.Name {
	// slashing transactions
	case UnjailMethod:
		bz, err = p.Unjail(ctx, method, stateDB, contract, args)
	// slashing queries
	case GetSigningInfoMethod:
		bz, err = p.GetSigningInfo(ctx, method, contract, args)
	case GetSigningInfosMethod:
		bz, err = p.GetSigningInfos(ctx, method, contract, args)
	case GetParamsMethod:
		bz, err = p.GetParams(ctx, method, contract, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
//
// Available slashing transactions are:
// - Unjail
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case UnjailMethod:
		return true
	default:
		return false
	}
}

// Logger returns a precompile-specific logger.
func (p Precompile) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("evm extension", "slashing")
}
