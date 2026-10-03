package bank

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Balance contains the amount for a corresponding ERC-20 contract address.
type Balance struct {
	ContractAddress common.Address
	Amount          *big.Int
}

// ParseBalancesArgs parses the call arguments for the bank Balances query.
func ParseBalancesArgs(args []interface{}) (sdk.AccAddress, error) {
	if len(args) != 1 {
		return nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(1), big.NewInt(int64(len(args))))
	}

	account, ok := args[0].(common.Address)
	if !ok {
		return nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, fmt.Sprintf("%v", args[0]))
	}

	return account.Bytes(), nil
}

// ParseSupplyOfArgs parses the call arguments for the bank SupplyOf query.
func ParseSupplyOfArgs(args []interface{}) (common.Address, error) {
	if len(args) != 1 {
		return common.Address{}, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(1), big.NewInt(int64(len(args))))
	}

	erc20Address, ok := args[0].(common.Address)
	if !ok {
		return common.Address{}, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, fmt.Sprintf("%v", args[0]))
	}

	return erc20Address, nil
}
