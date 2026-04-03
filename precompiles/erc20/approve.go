package erc20

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	sdkmath "cosmossdk.io/math"

	cmn "github.com/cosmos/evm/precompiles/common"

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

	owner := contract.Caller()

	allowance, err := p.erc20Keeper.GetAllowance(ctx, p.Address(), owner, spender)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrQueryFailed, ApproveMethod,
			fmt.Sprintf("%s: %v", fmt.Sprintf(ErrNoAllowanceForToken, p.tokenPair.Denom), err))
	}

	switch {
	case allowance.Sign() == 0 && amount != nil && amount.Sign() < 0:
		err = cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrInvalidAmount, "cannot approve negative values")
	case allowance.Sign() == 0 && amount != nil && amount.Sign() > 0:
		err = p.setAllowance(ctx, owner, spender, amount)
	case allowance.Sign() > 0 && amount != nil && amount.Sign() <= 0:
		if derr := p.erc20Keeper.DeleteAllowance(ctx, p.Address(), owner, spender); derr != nil {
			err = cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrQueryFailed, ApproveMethod, derr.Error())
		}
	case allowance.Sign() > 0 && amount != nil && amount.Sign() > 0:
		err = p.setAllowance(ctx, owner, spender, amount)
	}

	if err != nil {
		return nil, err
	}

	if err := p.EmitApprovalEvent(ctx, stateDB, owner, spender, amount); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, ApproveMethod, err.Error())
	}

	return method.Outputs.Pack(true)
}

func (p *Precompile) setAllowance(
	ctx sdk.Context,
	owner, spender common.Address,
	allowance *big.Int,
) error {
	if allowance.BitLen() > sdkmath.MaxBitLen {
		return cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrInvalidAmount, fmt.Sprintf(ErrIntegerOverflow, allowance))
	}

	if err := p.erc20Keeper.SetAllowance(ctx, p.Address(), owner, spender, allowance); err != nil {
		return cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrQueryFailed, ApproveMethod, err.Error())
	}
	return nil
}
