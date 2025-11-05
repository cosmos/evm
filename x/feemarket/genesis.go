package feemarket

import (
	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/x/feemarket/keeper"
	"github.com/cosmos/evm/x/feemarket/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes genesis state based on exported genesis
func InitGenesis(
	ctx sdk.Context,
	k keeper.Keeper,
	data types.GenesisState,
) []abci.ValidatorUpdate {
	err := k.SetParams(ctx, data.Params)
	if err != nil {
		panic(errorsmod.Wrap(err, "could not set parameters at genesis"))
	}

	k.SetBlockGasWanted(ctx, data.BlockGas)

	return []abci.ValidatorUpdate{}
}

// ExportGenesis exports genesis state of the fee market module
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *types.GenesisState {
	return &types.GenesisState{
		Params:   k.GetParams(ctx),
		BlockGas: k.GetBlockGasWanted(ctx),
	}
}
