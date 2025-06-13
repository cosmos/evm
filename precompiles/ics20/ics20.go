package ics20

import (
	"embed"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	channelkeeper "github.com/cosmos/ibc-go/v10/modules/core/04-channel/keeper"

	storetypes "cosmossdk.io/store/types"

	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

// PrecompileAddress of the ICS-20 EVM extension in hex format.
const PrecompileAddress = "0x0000000000000000000000000000000000000802"

var _ vm.PrecompiledContract = &Precompile{}

// Embed abi json file to the executable binary. Needed when importing as dependency.
//
//go:embed abi.json
var f embed.FS

type Precompile struct {
	cmn.Precompile
	stakingKeeper  stakingkeeper.Keeper
	transferKeeper transferkeeper.Keeper
	channelKeeper  *channelkeeper.Keeper
	evmKeeper      *evmkeeper.Keeper
}

// NewPrecompile creates a new ICS-20 Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	stakingKeeper stakingkeeper.Keeper,
	transferKeeper transferkeeper.Keeper,
	channelKeeper *channelkeeper.Keeper,
	evmKeeper *evmkeeper.Keeper,
) (*Precompile, error) {
	newAbi, err := cmn.LoadABI(f, "abi.json")
	if err != nil {
		return nil, err
	}

	p := &Precompile{
		Precompile: cmn.Precompile{
			ABI:                  newAbi,
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
		},
		transferKeeper: transferKeeper,
		channelKeeper:  channelKeeper,
		stakingKeeper:  stakingKeeper,
		evmKeeper:      evmKeeper,
	}

	// SetAddress defines the address of the ICS-20 compile contract.
	p.SetAddress(common.HexToAddress(evmtypes.ICS20PrecompileAddress))

	return p, nil
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

// Run executes the precompiled contract IBC transfer methods defined in the ABI.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readOnly bool) (bz []byte, err error) {
	ctx, stateDB, method, initialGas, args, err := p.RunSetup(evm, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	// This handles any out of gas errors that may occur during the execution of a precompile tx or query.
	// It avoids panics and returns the out of gas error so the EVM can continue gracefully.
	defer cmn.HandleGasError(ctx, contract, initialGas, &err)()

	switch method.Name {
	// ICS20 transactions
	case TransferMethod:
		bz, err = p.Transfer(ctx, contract, stateDB, method, args)
	// ICS20 queries
	case DenomMethod:
		bz, err = p.Denom(ctx, contract, method, args)
	case DenomsMethod:
		bz, err = p.Denoms(ctx, contract, method, args)
	case DenomHashMethod:
		bz, err = p.DenomHash(ctx, contract, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	if err != nil {
		return nil, err
	}

	cost := ctx.GasMeter().GasConsumed() - initialGas

	if !contract.UseGas(cost, nil, tracing.GasChangeCallPrecompiledContract) {
		return nil, vm.ErrOutOfGas
	}

	if err = p.AddJournalEntries(stateDB); err != nil {
		return nil, err
	}

	return bz, nil
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
//
// Available ics20 transactions are:
//   - Transfer
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case TransferMethod:
		return true
	default:
		return false
	}
}
