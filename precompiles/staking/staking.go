package staking

import (
	"embed"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

var _ vm.PrecompiledContract = &Precompile{}

// Embed abi json file to the executable binary. Needed when importing as dependency.
//
//go:embed abi.json
var f embed.FS

// Precompile defines the precompiled contract for staking.
type Precompile struct {
	cmn.Precompile
	stakingKeeper stakingkeeper.Keeper
}

// LoadABI loads the staking ABI from the embedded abi.json file
// for the staking precompile.
func LoadABI() (abi.ABI, error) {
	return cmn.LoadABI(f, "abi.json")
}

// NewPrecompile creates a new staking Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	stakingKeeper stakingkeeper.Keeper,
) (*Precompile, error) {
	abi, err := LoadABI()
	if err != nil {
		return nil, err
	}

	p := &Precompile{
		Precompile: cmn.Precompile{
			ABI:                  abi,
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
		},
		stakingKeeper: stakingKeeper,
	}
	// SetAddress defines the address of the staking precompiled contract.
	p.SetAddress(common.HexToAddress(evmtypes.StakingPrecompileAddress))

	return p, nil
}

// RequiredGas returns the required bare minimum gas to execute the precompile.
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

// Run executes the precompiled contract staking methods defined in the ABI.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readOnly bool) ([]byte, error) {
	return p.SetupAndRun(evm, contract, readOnly, p.IsTransaction, p.HandleMethod)
}

// HandleMethod handles the execution of each method
func (p Precompile) HandleMethod(ctx sdk.Context, contract *vm.Contract, stateDB vm.StateDB, method *abi.Method, args []interface{}) ([]byte, error) {
	switch method.Name {
	// Staking transactions
	case CreateValidatorMethod:
		return p.CreateValidator(ctx, contract, stateDB, method, args)
	case EditValidatorMethod:
		return p.EditValidator(ctx, contract, stateDB, method, args)
	case DelegateMethod:
		return p.Delegate(ctx, contract, stateDB, method, args)
	case UndelegateMethod:
		return p.Undelegate(ctx, contract, stateDB, method, args)
	case RedelegateMethod:
		return p.Redelegate(ctx, contract, stateDB, method, args)
	case CancelUnbondingDelegationMethod:
		return p.CancelUnbondingDelegation(ctx, contract, stateDB, method, args)
	// Staking queries
	case DelegationMethod:
		return p.Delegation(ctx, contract, method, args)
	case UnbondingDelegationMethod:
		return p.UnbondingDelegation(ctx, contract, method, args)
	case ValidatorMethod:
		return p.Validator(ctx, method, contract, args)
	case ValidatorsMethod:
		return p.Validators(ctx, method, contract, args)
	case RedelegationMethod:
		return p.Redelegation(ctx, method, contract, args)
	case RedelegationsMethod:
		return p.Redelegations(ctx, method, contract, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
//
// Available staking transactions are:
//   - CreateValidator
//   - EditValidator
//   - Delegate
//   - Undelegate
//   - Redelegate
//   - CancelUnbondingDelegation
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case CreateValidatorMethod,
		EditValidatorMethod,
		DelegateMethod,
		UndelegateMethod,
		RedelegateMethod,
		CancelUnbondingDelegationMethod:
		return true
	default:
		return false
	}
}

// Logger returns a precompile-specific logger.
func (p Precompile) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("evm extension", "staking")
}
