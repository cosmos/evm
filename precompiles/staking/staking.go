package staking

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

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
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

// Precompile defines the precompiled contract for staking.
type Precompile struct {
	cmn.Precompile

	abi.ABI
	stakingKeeper    cmn.StakingKeeper
	stakingMsgServer stakingtypes.MsgServer
	stakingQuerier   stakingtypes.QueryServer
	addrCdc          address.Codec
}

// NewPrecompile creates a new staking Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	stakingKeeper cmn.StakingKeeper,
	stakingMsgServer stakingtypes.MsgServer,
	stakingQuerier stakingtypes.QueryServer,
	bankKeeper cmn.BankKeeper,
	addrCdc address.Codec,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:           storetypes.KVGasConfig(),
			TransientKVGasConfig:  storetypes.TransientGasConfig(),
			ContractAddress:       common.HexToAddress(evmtypes.StakingPrecompileAddress),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:              ABI,
		stakingKeeper:    stakingKeeper,
		stakingMsgServer: stakingMsgServer,
		stakingQuerier:   stakingQuerier,
		addrCdc:          addrCdc,
	}
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
	// Staking transactions
	case CreateValidatorMethod:
		bz, err = p.CreateValidator(ctx, contract, stateDB, method, args)
	case EditValidatorMethod:
		bz, err = p.EditValidator(ctx, contract, stateDB, method, args)
	case DelegateMethod:
		bz, err = p.Delegate(ctx, contract, stateDB, method, args)
	case UndelegateMethod:
		bz, err = p.Undelegate(ctx, contract, stateDB, method, args)
	case RedelegateMethod:
		bz, err = p.Redelegate(ctx, contract, stateDB, method, args)
	case CancelUnbondingDelegationMethod:
		bz, err = p.CancelUnbondingDelegation(ctx, contract, stateDB, method, args)
	// Staking queries
	case DelegationMethod:
		bz, err = p.Delegation(ctx, contract, method, args)
	case UnbondingDelegationMethod:
		bz, err = p.UnbondingDelegation(ctx, contract, method, args)
	case ValidatorMethod:
		bz, err = p.Validator(ctx, method, contract, args)
	case ValidatorsMethod:
		bz, err = p.Validators(ctx, method, contract, args)
	case RedelegationMethod:
		bz, err = p.Redelegation(ctx, method, contract, args)
	case RedelegationsMethod:
		bz, err = p.Redelegations(ctx, method, contract, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
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
