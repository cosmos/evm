package erc20

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"
)

// EventTransfer defines the event data for the ERC20 Transfer events.
type EventTransfer struct {
	From  common.Address
	To    common.Address
	Value *big.Int
}

// EventApproval defines the event data for the ERC20 Approval events.
type EventApproval struct {
	Owner   common.Address
	Spender common.Address
	Value   *big.Int
}

// ParseTransferArgs parses the arguments from the transfer method and returns
// the destination address (to) and amount.
func ParseTransferArgs(args []interface{}) (
	to common.Address, amount *big.Int, err error,
) {
	if len(args) != 2 {
		return common.Address{}, nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(2), big.NewInt(int64(len(args))))
	}

	to, ok := args[0].(common.Address)
	if !ok {
		return common.Address{}, nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, fmt.Sprintf("%v", args[0]))
	}

	amount, ok = args[1].(*big.Int)
	if !ok {
		return common.Address{}, nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAmount, fmt.Sprintf("%v", args[1]))
	}

	return to, amount, nil
}

// ParseTransferFromArgs parses the arguments from the transferFrom method and returns
// the sender address (from), destination address (to) and amount.
func ParseTransferFromArgs(args []interface{}) (
	from, to common.Address, amount *big.Int, err error,
) {
	if len(args) != 3 {
		return common.Address{}, common.Address{}, nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(3), big.NewInt(int64(len(args))))
	}

	from, ok := args[0].(common.Address)
	if !ok {
		return common.Address{}, common.Address{}, nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, fmt.Sprintf("%v", args[0]))
	}

	to, ok = args[1].(common.Address)
	if !ok {
		return common.Address{}, common.Address{}, nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, fmt.Sprintf("%v", args[1]))
	}

	amount, ok = args[2].(*big.Int)
	if !ok {
		return common.Address{}, common.Address{}, nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAmount, fmt.Sprintf("%v", args[2]))
	}

	return from, to, amount, nil
}

// ParseApproveArgs parses the approval arguments and returns the spender address
// and amount.
func ParseApproveArgs(args []interface{}) (
	spender common.Address, amount *big.Int, err error,
) {
	if len(args) != 2 {
		return common.Address{}, nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(2), big.NewInt(int64(len(args))))
	}

	spender, ok := args[0].(common.Address)
	if !ok {
		return common.Address{}, nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, fmt.Sprintf("%v", args[0]))
	}

	amount, ok = args[1].(*big.Int)
	if !ok {
		return common.Address{}, nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAmount, fmt.Sprintf("%v", args[1]))
	}

	return spender, amount, nil
}

// ParseAllowanceArgs parses the allowance arguments and returns the owner and
// the spender addresses.
func ParseAllowanceArgs(args []interface{}) (
	owner, spender common.Address, err error,
) {
	if len(args) != 2 {
		return common.Address{}, common.Address{}, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(2), big.NewInt(int64(len(args))))
	}

	owner, ok := args[0].(common.Address)
	if !ok {
		return common.Address{}, common.Address{}, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, fmt.Sprintf("%v", args[0]))
	}

	spender, ok = args[1].(common.Address)
	if !ok {
		return common.Address{}, common.Address{}, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, fmt.Sprintf("%v", args[1]))
	}

	return owner, spender, nil
}

// ParseBalanceOfArgs parses the balanceOf arguments and returns the account address.
func ParseBalanceOfArgs(args []interface{}) (common.Address, error) {
	if len(args) != 1 {
		return common.Address{}, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(1), big.NewInt(int64(len(args))))
	}

	account, ok := args[0].(common.Address)
	if !ok {
		return common.Address{}, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, fmt.Sprintf("%v", args[0]))
	}

	return account, nil
}
