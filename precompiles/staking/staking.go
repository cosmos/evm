package staking

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/core/address"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

//go:generate go run ../cmd -input abi.json -output staking.abi.go

var _ vm.PrecompiledContract = &Precompile{}

// Precompile defines the precompiled contract for staking.
type Precompile struct {
	cmn.Precompile

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
		stakingKeeper:    stakingKeeper,
		stakingMsgServer: stakingMsgServer,
		stakingQuerier:   stakingQuerier,
		addrCdc:          addrCdc,
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
	// Staking transactions
	case CreateValidatorID:
		return cmn.RunWithStateDB(ctx, p.CreateValidator, input, stateDB, contract)
	case EditValidatorID:
		return cmn.RunWithStateDB(ctx, p.EditValidator, input, stateDB, contract)
	case DelegateID:
		return cmn.RunWithStateDB(ctx, p.Delegate, input, stateDB, contract)
	case UndelegateID:
		return cmn.RunWithStateDB(ctx, p.Undelegate, input, stateDB, contract)
	case RedelegateID:
		return cmn.RunWithStateDB(ctx, p.Redelegate, input, stateDB, contract)
	case CancelUnbondingDelegationID:
		return cmn.RunWithStateDB(ctx, p.CancelUnbondingDelegation, input, stateDB, contract)
	// Staking queries
	case DelegationID:
		return cmn.Run(ctx, p.Delegation, input)
	case UnbondingDelegationID:
		return cmn.Run(ctx, p.UnbondingDelegation, input)
	case ValidatorID:
		return cmn.Run(ctx, p.Validator, input)
	case ValidatorsID:
		return cmn.Run(ctx, p.Validators, input)
	case RedelegationID:
		return cmn.Run(ctx, p.Redelegation, input)
	case RedelegationsID:
		return cmn.Run(ctx, p.Redelegations, input)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, methodID)
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
func (Precompile) IsTransaction(method uint32) bool {
	switch method {
	case CreateValidatorID,
		EditValidatorID,
		DelegateID,
		UndelegateID,
		RedelegateID,
		CancelUnbondingDelegationID:
		return true
	default:
		return false
	}
}

// Logger returns a precompile-specific logger.
func (p Precompile) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("evm extension", "staking")
}
