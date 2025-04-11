package keeper

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func (k Keeper) GetAllowance(
	ctx sdk.Context,
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
) (*big.Int, error) {

	// TODO: implement this function
	return big.NewInt(0), nil
}

func (k Keeper) SetAllowance(
	ctx sdk.Context,
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
	value *big.Int,
) error {

	// TODO: implement this function
	return nil
}

func (k Keeper) DeleteAllowance(
	ctx sdk.Context,
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
) error {

	// TODO: implement this function
	return nil
}