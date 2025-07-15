package erc20factory

import (
	"embed"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	Erc20FactoryAddress = "0x0000000000000000000000000000000000000900"
	// GasCreate defines the gas required to create a new ERC20 Token Pair calculated from a ERC20 deploy transaction
	GasCreate = 3_000_000
	// GasCalculateAddress defines the gas required to calculate the address of a new ERC20 Token Pair
	GasCalculateAddress = 3_000
)

var _ vm.PrecompiledContract = &Precompile{}

// Embed abi json file to the executable binary. Needed when importing as dependency.
//
//go:embed abi.json
var f embed.FS

// Precompile defines the precompiled contract for Bech32 encoding.
type Precompile struct {
	cmn.Precompile
	erc20Keeper ERC20Keeper
	bankKeeper  BankKeeper
}

// NewPrecompile creates a new bech32 Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(erc20Keeper ERC20Keeper, bankKeeper BankKeeper) (*Precompile, error) {
	newABI, err := cmn.LoadABI(f, "abi.json")
	if err != nil {
		return nil, err
	}

	p := &Precompile{
		Precompile: cmn.Precompile{
			ABI:                  newABI,
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
		},
		erc20Keeper: erc20Keeper,
		bankKeeper:  bankKeeper,
	}

	// SetAddress defines the address of the distribution compile contract.
	p.SetAddress(common.HexToAddress(Erc20FactoryAddress))
	return p, nil
}

// Address defines the address of the bech32 precompiled contract.
func (Precompile) Address() common.Address {
	return common.HexToAddress(Erc20FactoryAddress)
}

// RequiredGas calculates the contract gas use.
func (p Precompile) RequiredGas(input []byte) uint64 {
	// NOTE: This check avoid panicking when trying to decode the method ID
	if len(input) < 4 {
		return 0
	}

	methodID := input[:4]
	method, err := p.MethodById(methodID)
	if err != nil {
		return 0
	}

	switch method.Name {
	// ERC-20 transactions
	case CreateMethod:
		return GasCreate
	case CalculateAddressMethod:
		return GasCalculateAddress
	default:
		return 0
	}
}

// Run executes the precompiled contract bech32 methods defined in the ABI.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readOnly bool) (bz []byte, err error) {
	ctx, stateDB, method, initialGas, args, err := p.RunSetup(evm, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}
	// This handles any out of gas errors that may occur during the execution of a precompile query.
	// It avoids panics and returns the out of gas error so the EVM can continue gracefully.
	defer cmn.HandleGasError(ctx, contract, initialGas, &err)()

	bz, err = p.HandleMethod(ctx, contract, stateDB, method, args)
	if err != nil {
		return nil, err
	}

	cost := ctx.GasMeter().GasConsumed() - initialGas

	if !contract.UseGas(cost, nil, tracing.GasChangeCallPrecompiledContract) {
		return nil, vm.ErrOutOfGas
	}

	// Process the native balance changes after the method execution.
	err = p.GetBalanceHandler().AfterBalanceChange(ctx, stateDB)
	if err != nil {
		return nil, err
	}

	return bz, nil
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
//
// Available ERC20 Factory transactions are:
//   - Create
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case CreateMethod:
		return true
	default:
		return false
	}
}

// HandleMethod handles the execution of each of the ERC-20 Factory methods.
func (p *Precompile) HandleMethod(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) (bz []byte, err error) {
	switch method.Name {
	// ERC-20 Factory transactions
	case CreateMethod:
		bz, err = p.Create(ctx, stateDB, method, contract.Caller(), args)
	// ERC-20 Factory queries
	case CalculateAddressMethod:
		bz, err = p.CalculateAddress(method, contract.Caller(), args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}
	return bz, err
}
