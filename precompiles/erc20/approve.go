package erc20

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/x/vm/core/vm"

	sdkerrors "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Approve sets the given amount as the allowance of the spender address over
// the caller’s tokens. It returns a boolean value indicating whether the
// operation succeeded and emits the Approval event on success.
//
// The Approve method handles 4 cases:
//  1. no allowance, amount negative -> return error
//  2. no allowance, amount positive -> create a new allowance
//  3. allowance exists, amount 0 or negative -> delete allowance
//  4. allowance exists, amount positive -> update allowance
//  5. no allowance, amount 0 -> no-op but still emit Approval event
func (p Precompile) Approve(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	spender, amount, err := ParseApproveArgs(args)
	if err != nil {
		return nil, err
	}

	grantee := spender
	granter := contract.CallerAddress

	// NOTE: We do not support approvals if the grantee is the granter.
	// This is different from the ERC20 standard but there is no reason to
	// do so, since in that case the grantee can just transfer the tokens
	// without allowance.
	if bytes.Equal(grantee.Bytes(), granter.Bytes()) {
		return nil, ErrSpenderIsOwner
	}

	// TODO: owner should be the owner of the contract
	allowance, err := p.erc20Keeper.GetAllowance(ctx, p.Address(), grantee, granter)
	if err != nil {
		return nil, sdkerrors.Wrap(err, fmt.Sprintf(ErrNoAllowanceForToken, p.tokenPair.Denom))
	}

	switch {
	case allowance == nil && amount != nil && amount.Sign() < 0:
		// case 1: no allowance, amount 0 or negative -> error
		err = ErrNegativeAmount
	case allowance == nil && amount != nil && amount.Sign() > 0:
		// case 2: no allowance, amount positive -> create a new allowance
		err = p.erc20Keeper.SetAllowance(ctx, p.Address(), grantee, granter, amount)
	case allowance != nil && amount != nil && amount.Sign() <= 0:
		// case 3: allowance exists, amount 0 or negative -> remove from spend limit and delete allowance if no spend limit left
		err = p.erc20Keeper.DeleteAllowance(ctx, p.Address(), grantee, granter)
	case allowance != nil && amount != nil && amount.Sign() > 0:
		// case 4: allowance exists, amount positive -> update allowance
		err = p.erc20Keeper.SetAllowance(ctx, p.Address(), grantee, granter, amount)
	}

	if err != nil {
		return nil, err
	}

	// TODO: check owner?
	if err := p.EmitApprovalEvent(ctx, stateDB, p.Address(), spender, amount); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// IncreaseAllowance increases the allowance of the spender address over
// the caller’s tokens by the given added value. It returns a boolean value
// indicating whether the operation succeeded and emits the Approval event on
// success.
//
// The IncreaseAllowance method handles 3 cases:
//  1. addedValue 0 or negative -> return error
//  2. no allowance, addedValue positive -> create a new allowance
//  3. allowance exists, addedValue positive -> update allowance
func (p Precompile) IncreaseAllowance(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	spender, addedValue, err := ParseApproveArgs(args)
	if err != nil {
		return nil, err
	}

	grantee := spender
	granter := contract.CallerAddress

	if bytes.Equal(grantee.Bytes(), granter.Bytes()) {
		return nil, ErrSpenderIsOwner
	}

	// TODO: owner should be the owner of the contract
	allowance, err := p.erc20Keeper.GetAllowance(ctx, p.Address(), grantee, granter)
	if err != nil {
		return nil, sdkerrors.Wrap(err, fmt.Sprintf(ErrNoAllowanceForToken, p.tokenPair.Denom))
	}

	var amount *big.Int
	switch {
	case addedValue != nil && addedValue.Sign() <= 0:
		// case 1: addedValue 0 or negative -> error
		// TODO: (@fedekunze) check if this is correct by comparing behavior with
		// regular ERC20
		err = ErrIncreaseNonPositiveValue
	case allowance == nil && addedValue != nil && addedValue.Sign() > 0:
		// case 2: no allowance, amount positive -> create a new allowance
		amount = addedValue
		err = p.erc20Keeper.SetAllowance(ctx, p.Address(), grantee, granter, addedValue)
	case allowance != nil && addedValue != nil && addedValue.Sign() > 0:
		// case 3: allowance exists, amount positive -> update allowance
		amount, err = p.increaseAllowance(ctx, grantee, granter, allowance, addedValue)
	}

	if err != nil {
		return nil, err
	}

	// TODO: check owner?
	if err := p.EmitApprovalEvent(ctx, stateDB, p.Address(), spender, amount); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// DecreaseAllowance decreases the allowance of the spender address over
// the caller’s tokens by the given subtracted value. It returns a boolean value
// indicating whether the operation succeeded and emits the Approval event on
// success.
//
// The DecreaseAllowance method handles 4 cases:
//  1. subtractedValue 0 or negative -> return error
//  2. no allowance -> return error
//  3. allowance exists, subtractedValue positive and subtractedValue less than allowance -> update allowance
//  4. allowance exists, subtractedValue positive and subtractedValue equal to allowance -> delete allowance
//  5. allowance exists, subtractedValue positive but no allowance for given denomination -> return error
//  6. allowance exists, subtractedValue positive and subtractedValue higher than allowance -> return error
func (p Precompile) DecreaseAllowance(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	spender, subtractedValue, err := ParseApproveArgs(args)
	if err != nil {
		return nil, err
	}

	grantee := spender
	granter := contract.CallerAddress

	if bytes.Equal(grantee.Bytes(), granter.Bytes()) {
		return nil, ErrSpenderIsOwner
	}
	// TODO: owner should be the owner of the contract

	allowance, err := p.erc20Keeper.GetAllowance(ctx, p.Address(), grantee, granter)
	if err != nil {
		return nil, sdkerrors.Wrap(err, fmt.Sprintf(ErrNoAllowanceForToken, p.tokenPair.Denom))
	}

	// TODO: (@fedekunze) check if this is correct by comparing behavior with
	// regular ERC-20
	var amount *big.Int
	switch {
	case subtractedValue != nil && subtractedValue.Sign() <= 0:
		// case 1. subtractedValue 0 or negative -> return error
		err = ErrDecreaseNonPositiveValue
	case allowance == nil:
		// case 2. no allowance -> return error
		err = sdkerrors.Wrap(err, fmt.Sprintf(ErrNoAllowanceForToken, p.tokenPair.Denom))
	case subtractedValue != nil && subtractedValue.Cmp(allowance) < 0:
		// case 3. subtractedValue positive and subtractedValue less than allowance -> update allowance
		amount, err = p.decreaseAllowance(ctx, grantee, granter, allowance, subtractedValue)
	case subtractedValue != nil && subtractedValue.Cmp(allowance) == 0:
		// case 4. subtractedValue positive and subtractedValue equal to allowance -> remove spend limit for token and delete allowance if no other denoms are approved for
		err = p.erc20Keeper.DeleteAllowance(ctx, p.Address(), grantee, granter)
		amount = nil
	case subtractedValue != nil && allowance.Sign() == 0:
		// case 5. subtractedValue positive but no allowance for given denomination -> return error
		err = fmt.Errorf(ErrNoAllowanceForToken, p.tokenPair.Denom)
	case subtractedValue != nil && subtractedValue.Cmp(allowance) > 0:
		// case 6. subtractedValue positive and subtractedValue higher than allowance -> return error
		err = ConvertErrToERC20Error(fmt.Errorf(ErrSubtractMoreThanAllowance, p.tokenPair.Denom, subtractedValue, allowance))
	}

	if err != nil {
		return nil, err
	}

	// TODO: check owner?
	if err := p.EmitApprovalEvent(ctx, stateDB, p.Address(), spender, amount); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func (p Precompile) increaseAllowance(
	ctx sdk.Context,
	grantee, granter common.Address,
	allowance, addedValue *big.Int,
) (amount *big.Int, err error) {
	sdkAllowance := sdkmath.NewIntFromBigInt(allowance)
	sdkAddedValue := sdkmath.NewIntFromBigInt(addedValue)
	amount, overflow := cmn.SafeAdd(sdkAllowance, sdkAddedValue)
	if overflow {
		return nil, ConvertErrToERC20Error(errors.New(cmn.ErrIntegerOverflow))
	}

	if err := p.erc20Keeper.SetAllowance(ctx, p.Address(), grantee, granter, amount); err != nil {
		return nil, err
	}

	return amount, nil
}

func (p Precompile) decreaseAllowance(
	ctx sdk.Context,
	grantee, granter common.Address,
	allowance, subtractedValue *big.Int,
) (amount *big.Int, err error) {
	amount = new(big.Int).Sub(allowance, subtractedValue)
	// NOTE: Safety check only since this is checked in the DecreaseAllowance method already.
	if amount.Sign() < 0 {
		return nil, ConvertErrToERC20Error(fmt.Errorf(ErrSubtractMoreThanAllowance, p.tokenPair.Denom, subtractedValue, allowance))
	}

	if err := p.erc20Keeper.SetAllowance(ctx, p.Address(), grantee, granter, amount); err != nil {
		return nil, err
	}

	return amount, nil
}
