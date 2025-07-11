package werc20

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	erc20types "github.com/cosmos/evm/x/erc20/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type Erc20Keeper interface {
	GetAllowance(ctx sdk.Context, erc20 common.Address, owner common.Address, spender common.Address) (*big.Int, error)
	SetAllowance(ctx sdk.Context, erc20 common.Address, owner common.Address, spender common.Address, value *big.Int) error
	DeleteAllowance(ctx sdk.Context, erc20 common.Address, owner common.Address, spender common.Address) error
	GetParams(ctx sdk.Context) erc20types.Params
}
