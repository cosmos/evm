package erc20

import (
	"fmt"
	"math/big"

	"github.com/cosmos/evm/x/erc20/keeper"
	"github.com/cosmos/evm/x/erc20/types"
	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
)

// InitGenesis import module genesis
func InitGenesis(
	ctx sdk.Context,
	k keeper.Keeper,
	accountKeeper authkeeper.AccountKeeper,
	data types.GenesisState,
) {
	err := k.SetParams(ctx, data.Params)
	if err != nil {
		panic(fmt.Errorf("error setting params %s", err))
	}

	// ensure erc20 module account is set on genesis
	if acc := accountKeeper.GetModuleAccount(ctx, types.ModuleName); acc == nil {
		// NOTE: shouldn't occur
		panic("the erc20 module account has not been set")
	}

	for _, pair := range data.TokenPairs {
		k.SetToken(ctx, pair)
	}

	var erc20, owner, spender common.Address
	var value *big.Int
	for _, allowance := range data.Allowances {
		erc20 = common.HexToAddress(allowance.Erc20Address)
		owner = common.HexToAddress(allowance.Owner)
		spender = common.HexToAddress(allowance.Spender)
		value = allowance.Value.BigInt()
		err := k.SetAllowance(ctx, erc20, owner, spender, value)
		if err != nil {
			if types.ErrERC20TokenPairDisabled.Is(err) {
				// NOTES: When SetAllowance is called from the ERC20 precompile, this case is treated as an error,
				// but during GenesisState initialization, it’s a valid case, so it is allowed to pass.
				continue
			}
			panic(fmt.Errorf("error setting allowance %s", err))
		}
	}
}

// ExportGenesis export module status
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	return &types.GenesisState{
		Params:     k.GetParams(ctx),
		TokenPairs: k.GetTokenPairs(ctx),
		Allowances: k.GetAllowances(ctx),
	}
}
