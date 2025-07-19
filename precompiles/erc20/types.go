package erc20

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	erc20types "github.com/cosmos/evm/x/erc20/types"
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
		return common.Address{}, nil, fmt.Errorf("invalid number of arguments; expected 2; got: %d", len(args))
	}

	to, ok := args[0].(common.Address)
	if !ok {
		return common.Address{}, nil, fmt.Errorf("invalid to address: %v", args[0])
	}

	amount, ok = args[1].(*big.Int)
	if !ok {
		return common.Address{}, nil, fmt.Errorf("invalid amount: %v", args[1])
	}

	return to, amount, nil
}

// ParseTransferFromArgs parses the arguments from the transferFrom method and returns
// the sender address (from), destination address (to) and amount.
func ParseTransferFromArgs(args []interface{}) (
	from, to common.Address, amount *big.Int, err error,
) {
	if len(args) != 3 {
		return common.Address{}, common.Address{}, nil, fmt.Errorf("invalid number of arguments; expected 3; got: %d", len(args))
	}

	from, ok := args[0].(common.Address)
	if !ok {
		return common.Address{}, common.Address{}, nil, fmt.Errorf("invalid from address: %v", args[0])
	}

	to, ok = args[1].(common.Address)
	if !ok {
		return common.Address{}, common.Address{}, nil, fmt.Errorf("invalid to address: %v", args[1])
	}

	amount, ok = args[2].(*big.Int)
	if !ok {
		return common.Address{}, common.Address{}, nil, fmt.Errorf("invalid amount: %v", args[2])
	}

	return from, to, amount, nil
}

// ParseApproveArgs parses the approval arguments and returns the spender address
// and amount.
func ParseApproveArgs(args []interface{}) (
	spender common.Address, amount *big.Int, err error,
) {
	if len(args) != 2 {
		return common.Address{}, nil, fmt.Errorf("invalid number of arguments; expected 2; got: %d", len(args))
	}

	spender, ok := args[0].(common.Address)
	if !ok {
		return common.Address{}, nil, fmt.Errorf("invalid spender address: %v", args[0])
	}

	amount, ok = args[1].(*big.Int)
	if !ok {
		return common.Address{}, nil, fmt.Errorf("invalid amount: %v", args[1])
	}

	return spender, amount, nil
}

// ParseAllowanceArgs parses the allowance arguments and returns the owner and
// the spender addresses.
func ParseAllowanceArgs(args []interface{}) (
	owner, spender common.Address, err error,
) {
	if len(args) != 2 {
		return common.Address{}, common.Address{}, fmt.Errorf("invalid number of arguments; expected 2; got: %d", len(args))
	}

	owner, ok := args[0].(common.Address)
	if !ok {
		return common.Address{}, common.Address{}, fmt.Errorf("invalid owner address: %v", args[0])
	}

	spender, ok = args[1].(common.Address)
	if !ok {
		return common.Address{}, common.Address{}, fmt.Errorf("invalid spender address: %v", args[1])
	}

	return owner, spender, nil
}

// ParseBalanceOfArgs parses the balanceOf arguments and returns the account address.
func ParseBalanceOfArgs(args []interface{}) (common.Address, error) {
	if len(args) != 1 {
		return common.Address{}, fmt.Errorf("invalid number of arguments; expected 1; got: %d", len(args))
	}

	account, ok := args[0].(common.Address)
	if !ok {
		return common.Address{}, fmt.Errorf("invalid account address: %v", args[0])
	}

	return account, nil
}

// NewGetParamsRequest creates a new request for the ERC20 parameters query.
func NewGetParamsRequest(args []interface{}) (*erc20types.QueryParamsRequest, error) {
	return &erc20types.QueryParamsRequest{}, nil
}

func ConvertStringsToAddresses(strings []string) ([]common.Address, error) {
	addresses := make([]common.Address, len(strings))
	for i, str := range strings {
		addr := common.HexToAddress(str)
		if addr == (common.Address{}) {
			return nil, fmt.Errorf("invalid address: %s", str)
		}
		addresses[i] = addr
	}
	return addresses, nil
}

// GetParamsOutput contains the output data for the ERC20 parameters query
type GetParamsOutput struct {
	EnableErc20                bool             `abi:"enableErc20"`
	NativePrecompiles          []common.Address `abi:"nativePrecompiles"`
	DynamicPrecompiles         []common.Address `abi:"dynamicPrecompiles"`
	PermissionlessRegistration bool             `abi:"permissionlessRegistration"`
}

// FromResponse populates the GetParamsOutput from the ERC20 Params
func (o *GetParamsOutput) FromResponse(params erc20types.Params) (*GetParamsOutput, error) {
	o.EnableErc20 = params.EnableErc20

	nativePrecompiles, err := ConvertStringsToAddresses(params.NativePrecompiles)
	if err != nil {
		return nil, fmt.Errorf("failed to convert native precompiles: %w", err)
	}
	o.NativePrecompiles = nativePrecompiles

	dynamicPrecompiles, err := ConvertStringsToAddresses(params.DynamicPrecompiles)
	if err != nil {
		return nil, fmt.Errorf("failed to convert dynamic precompiles: %w", err)
	}
	o.DynamicPrecompiles = dynamicPrecompiles

	o.PermissionlessRegistration = params.PermissionlessRegistration
	return o, nil
}

// Pack packs a given slice of abi arguments into a byte array.
func (o *GetParamsOutput) Pack(args abi.Arguments) ([]byte, error) {
	return args.Pack(o.EnableErc20, o.NativePrecompiles, o.DynamicPrecompiles, o.PermissionlessRegistration)
}
