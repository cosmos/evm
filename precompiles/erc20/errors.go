package erc20

import (
	"errors"
	"strings"

	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/evm/ibc"
	cmn "github.com/cosmos/evm/precompiles/common"
)

// Errors that have formatted information are defined here as a string.
const (
	ErrIntegerOverflow           = "amount %s causes integer overflow"
	ErrInvalidOwner              = "invalid from address: %s"
	ErrInvalidReceiver           = "invalid to address: %s"
	ErrNoAllowanceForToken       = "allowance for token %s does not exist"
	ErrSubtractMoreThanAllowance = "subtracted value cannot be greater than existing allowance for denom %s: %s > %s"
	ErrCannotReceiveFunds        = "cannot receive funds, received: %s"
)

var (
	// Precompile errors
	ErrDecreaseNonPositiveValue = errors.New("cannot decrease allowance with non-positive values")
	ErrIncreaseNonPositiveValue = errors.New("cannot increase allowance with non-positive values")
	ErrNegativeAmount           = errors.New("cannot approve negative values")
	ErrSpenderIsOwner           = errors.New("spender cannot be the owner")

	// ERC20 errors
	ErrDecreasedAllowanceBelowZero  = errors.New("ERC20: decreased allowance below zero")
	ErrInsufficientAllowance        = errors.New("ERC20: insufficient allowance")
	ErrTransferAmountExceedsBalance = errors.New("ERC20: transfer amount exceeds balance")
)

// ConvertErrToERC20Error is a helper function which maps errors raised by the Cosmos SDK stack
// to the corresponding errors which are raised by an ERC20 contract.
//
// TODO: Create the full RevertError types instead of just the standard error type.
//
// TODO: Return ERC-6093 compliant errors.
func ConvertErrToERC20Error(err error) error {
	switch {
	case strings.Contains(err.Error(), "spendable balance"):
		return ErrTransferAmountExceedsBalance
	case strings.Contains(err.Error(), "requested amount is more than spend limit"):
		return ErrInsufficientAllowance
	case strings.Contains(err.Error(), "subtracted value cannot be greater than existing allowance"):
		return ErrDecreasedAllowanceBelowZero
	case strings.Contains(err.Error(), cmn.ErrIntegerOverflow):
		return vm.ErrExecutionReverted
	case errors.Is(err, ibc.ErrNoIBCVoucherDenom) ||
		errors.Is(err, ibc.ErrDenomNotFound) ||
		strings.Contains(err.Error(), "invalid base denomination") ||
		strings.Contains(err.Error(), "display denomination not found") ||
		strings.Contains(err.Error(), "invalid decimals"):
		// NOTE: These are the cases when trying to query metadata of a contract, which has no metadata available.
		// The ERC20 contract raises an "execution reverted" error, without any further information here, which we
		// reproduce (even though it's less verbose than the actual error).
		return vm.ErrExecutionReverted
	default:
		return err
	}
}
