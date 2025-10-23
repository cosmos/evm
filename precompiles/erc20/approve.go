package erc20

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	sdkerrors "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Approve sets the given amount as the allowance of the spender address over
// the caller's tokens. It returns a boolean value indicating whether the
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
	args ApproveCall,
	stateDB vm.StateDB,
	contract *vm.Contract,
) (*ApproveReturn, error) {
	owner := contract.Caller()

	// TODO: owner should be the owner of the contract
	allowance, err := p.erc20Keeper.GetAllowance(ctx, p.Address(), owner, args.Spender)
	if err != nil {
		return nil, sdkerrors.Wrap(err, fmt.Sprintf(ErrNoAllowanceForToken, p.tokenPair.Denom))
	}

	switch {
	case allowance.Sign() == 0 && args.Amount != nil && args.Amount.Sign() < 0:
		// case 1: no allowance, amount 0 or negative -> error
		err = ErrNegativeAmount
	case allowance.Sign() == 0 && args.Amount != nil && args.Amount.Sign() > 0:
		// case 2: no allowance, amount positive -> create a new allowance
		err = p.setAllowance(ctx, owner, args.Spender, args.Amount)
	case allowance.Sign() > 0 && args.Amount != nil && args.Amount.Sign() <= 0:
		// case 3: allowance exists, amount 0 or negative -> remove from spend limit and delete allowance if no spend limit left
		err = p.erc20Keeper.DeleteAllowance(ctx, p.Address(), owner, args.Spender)
	case allowance.Sign() > 0 && args.Amount != nil && args.Amount.Sign() > 0:
		// case 4: allowance exists, amount positive -> update allowance
		err = p.setAllowance(ctx, owner, args.Spender, args.Amount)
	}

	if err != nil {
		return nil, err
	}

	if err := p.EmitApprovalEvent(ctx, stateDB, owner, args.Spender, args.Amount); err != nil {
		return nil, err
	}

	return &ApproveReturn{Field1: true}, nil
}

func (p Precompile) setAllowance(
	ctx sdk.Context,
	owner, spender common.Address,
	allowance *big.Int,
) error {
	if allowance.BitLen() > sdkmath.MaxBitLen {
		return fmt.Errorf(ErrIntegerOverflow, allowance)
	}

	return p.erc20Keeper.SetAllowance(ctx, p.Address(), owner, spender, allowance)
}
