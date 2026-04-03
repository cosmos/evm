package staking

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
	cmn "github.com/cosmos/evm/precompiles/common"
)

const (
	// CreateValidatorMethod defines the ABI method name for the staking create validator transaction
	CreateValidatorMethod = "createValidator"
	// EditValidatorMethod defines the ABI method name for the staking edit validator transaction
	EditValidatorMethod = "editValidator"
	// DelegateMethod defines the ABI method name for the staking Delegate
	// transaction.
	DelegateMethod = "delegate"
	// UndelegateMethod defines the ABI method name for the staking Undelegate
	// transaction.
	UndelegateMethod = "undelegate"
	// RedelegateMethod defines the ABI method name for the staking Redelegate
	// transaction.
	RedelegateMethod = "redelegate"
	// CancelUnbondingDelegationMethod defines the ABI method name for the staking
	// CancelUnbondingDelegation transaction.
	CancelUnbondingDelegationMethod = "cancelUnbondingDelegation"
)

// CreateValidator performs create validator.
func (p Precompile) CreateValidator(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, SolidityErrBondDenomQueryFailed, err.Error())
	}
	msg, validatorHexAddr, err := NewMsgCreateValidator(args, bondDenom, p.addrCdc)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"commission", msg.Commission.String(),
		"min_self_delegation", msg.MinSelfDelegation.String(),
		"validator_address", validatorHexAddr.String(),
		"pubkey", msg.Pubkey.String(),
		"value", msg.Value.Amount.String(),
	)

	msgSender := contract.Caller()

	// We won't allow calls from smart contracts
	// unless they are EIP-7702 delegated accounts
	code := stateDB.GetCode(msgSender)
	_, delegated := ethtypes.ParseDelegation(code)
	if len(code) > 0 && !delegated {
		// call by contract method
		return nil, cmn.NewRevertWithSolidityError(p.ABI, SolidityErrCannotCallFromContract, msgSender, big.NewInt(int64(len(code))), delegated)
	}

	if msgSender != validatorHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, validatorHexAddr)
	}

	// Execute the transaction using the message server
	if _, err = p.stakingMsgServer.CreateValidator(ctx, msg); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, CreateValidatorMethod, err.Error())
	}

	// Here we don't add journal entries here because calls from
	// smart contracts are not supported at the moment for this method.

	// Emit the event for the create validator transaction
	if err = p.EmitCreateValidatorEvent(ctx, stateDB, msg, validatorHexAddr); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, CreateValidatorMethod, err.Error())
	}

	return method.Outputs.Pack(true)
}

// EditValidator performs edit validator.
func (p Precompile) EditValidator(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, validatorHexAddr, err := NewMsgEditValidator(args)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"validator_address", msg.ValidatorAddress,
		"commission_rate", msg.CommissionRate,
		"min_self_delegation", msg.MinSelfDelegation,
	)

	msgSender := contract.Caller()

	// We won't allow calls from smart contracts
	// unless they are EIP-7702 delegated accounts
	code := stateDB.GetCode(msgSender)
	_, delegated := ethtypes.ParseDelegation(code)
	if len(code) > 0 && !delegated {
		// call by contract method
		return nil, cmn.NewRevertWithSolidityError(p.ABI, SolidityErrCannotCallFromContract, msgSender, big.NewInt(int64(len(code))), delegated)
	}

	if msgSender != validatorHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, validatorHexAddr)
	}

	// Execute the transaction using the message server
	if _, err = p.stakingMsgServer.EditValidator(ctx, msg); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, EditValidatorMethod, err.Error())
	}

	// Emit the event for the edit validator transaction
	if err = p.EmitEditValidatorEvent(ctx, stateDB, msg, validatorHexAddr); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, EditValidatorMethod, err.Error())
	}

	return method.Outputs.Pack(true)
}

// Delegate performs a delegation of coins from a delegator to a validator.
func (p *Precompile) Delegate(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, SolidityErrBondDenomQueryFailed, err.Error())
	}
	msg, delegatorHexAddr, err := NewMsgDelegate(args, bondDenom, p.addrCdc)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf(
			"{ delegator_address: %s, validator_address: %s, amount: %s }",
			delegatorHexAddr,
			msg.ValidatorAddress,
			msg.Amount.Amount,
		),
	)

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, delegatorHexAddr)
	}

	// Execute the transaction using the message server
	if _, err = p.stakingMsgServer.Delegate(ctx, msg); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, DelegateMethod, err.Error())
	}

	// Emit the event for the delegate transaction
	if err = p.EmitDelegateEvent(ctx, stateDB, msg, delegatorHexAddr); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, DelegateMethod, err.Error())
	}

	return method.Outputs.Pack(true)
}

// Undelegate performs the undelegation of coins from a validator for a delegate.
// The provided amount cannot be negative. This is validated in the msg.ValidateBasic() function.
func (p Precompile) Undelegate(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, SolidityErrBondDenomQueryFailed, err.Error())
	}
	msg, delegatorHexAddr, err := NewMsgUndelegate(args, bondDenom, p.addrCdc)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf(
			"{ delegator_address: %s, validator_address: %s, amount: %s }",
			delegatorHexAddr,
			msg.ValidatorAddress,
			msg.Amount.Amount,
		),
	)

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, delegatorHexAddr)
	}

	// Execute the transaction using the message server
	res, err := p.stakingMsgServer.Undelegate(ctx, msg)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, UndelegateMethod, err.Error())
	}

	// Emit the event for the undelegate transaction
	if err = p.EmitUnbondEvent(ctx, stateDB, msg, delegatorHexAddr, res.CompletionTime.UTC().Unix()); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, UndelegateMethod, err.Error())
	}

	return method.Outputs.Pack(res.CompletionTime.UTC().Unix())
}

// Redelegate performs a redelegation of coins for a delegate from a source validator
// to a destination validator.
// The provided amount cannot be negative. This is validated in the msg.ValidateBasic() function.
func (p Precompile) Redelegate(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, SolidityErrBondDenomQueryFailed, err.Error())
	}
	msg, delegatorHexAddr, err := NewMsgRedelegate(args, bondDenom, p.addrCdc)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf(
			"{ delegator_address: %s, validator_src_address: %s, validator_dst_address: %s, amount: %s }",
			delegatorHexAddr,
			msg.ValidatorSrcAddress,
			msg.ValidatorDstAddress,
			msg.Amount.Amount,
		),
	)

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, delegatorHexAddr)
	}

	res, err := p.stakingMsgServer.BeginRedelegate(ctx, msg)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, RedelegateMethod, err.Error())
	}

	if err = p.EmitRedelegateEvent(ctx, stateDB, msg, delegatorHexAddr, res.CompletionTime.UTC().Unix()); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, RedelegateMethod, err.Error())
	}

	return method.Outputs.Pack(res.CompletionTime.UTC().Unix())
}

// CancelUnbondingDelegation will cancel the unbonding of a delegation and delegate
// back to the validator being unbonded from.
// The provided amount cannot be negative. This is validated in the msg.ValidateBasic() function.
func (p Precompile) CancelUnbondingDelegation(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, SolidityErrBondDenomQueryFailed, err.Error())
	}
	msg, delegatorHexAddr, err := NewMsgCancelUnbondingDelegation(args, bondDenom, p.addrCdc)
	if err != nil {
		return nil, err
	}

	p.Logger(ctx).Debug(
		"tx called",
		"method", method.Name,
		"args", fmt.Sprintf(
			"{ delegator_address: %s, validator_address: %s, amount: %s, creation_height: %d }",
			delegatorHexAddr,
			msg.ValidatorAddress,
			msg.Amount.Amount,
			msg.CreationHeight,
		),
	)

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, delegatorHexAddr)
	}

	if _, err = p.stakingMsgServer.CancelUnbondingDelegation(ctx, msg); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, CancelUnbondingDelegationMethod, err.Error())
	}

	if err = p.EmitCancelUnbondingDelegationEvent(ctx, stateDB, msg, delegatorHexAddr); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, CancelUnbondingDelegationMethod, err.Error())
	}

	return method.Outputs.Pack(true)
}
