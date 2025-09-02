package epixmint

import (
	"github.com/cosmos/evm/x/epixmint/keeper"
	"github.com/cosmos/evm/x/epixmint/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes the epixmint module's state from a provided genesis
// state.
func InitGenesis(ctx sdk.Context, keeper keeper.Keeper, genState *types.GenesisState) {
	// Ensure epixmint module account is set
	if addr := keeper.GetModuleAddress(types.ModuleName); addr == nil {
		panic("the epixmint module account has not been set")
	}

	// Ensure distribution module account is set (needed for staking rewards)
	if addr := keeper.GetModuleAddress("distribution"); addr == nil {
		panic("the distribution module account has not been set")
	}

	// Set the parameters
	if err := keeper.SetParams(ctx, genState.Params); err != nil {
		panic(err)
	}
}

// ExportGenesis returns the epixmint module's exported genesis.
func ExportGenesis(ctx sdk.Context, keeper keeper.Keeper) *types.GenesisState {
	params := keeper.GetParams(ctx)

	return &types.GenesisState{
		Params: params,
	}
}
