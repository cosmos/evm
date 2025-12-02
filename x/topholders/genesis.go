package topholders

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/evm/x/topholders/keeper"
	"github.com/cosmos/evm/x/topholders/types"
)

// GenesisState defines the topholders module's genesis state.
type GenesisState struct {
	// Cache contains the initial top holders cache data
	Cache *types.TopHoldersCache `json:"cache,omitempty"`
}

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Cache: nil, // Start with empty cache
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	if gs.Cache != nil {
		return gs.Cache.Validate()
	}
	return nil
}

// InitGenesis initializes the topholders module's state from a provided genesis
// state.
func InitGenesis(ctx sdk.Context, k keeper.Keeper, genState *GenesisState) {
	if genState.Cache != nil {
		if err := k.SetTopHoldersCache(ctx, *genState.Cache); err != nil {
			panic(err)
		}
	}
}

// ExportGenesis returns the topholders module's exported genesis.
func ExportGenesis(ctx sdk.Context, k keeper.Keeper) *GenesisState {
	cache, found := k.GetTopHoldersCache(ctx)
	if !found {
		return DefaultGenesis()
	}

	return &GenesisState{
		Cache: &cache,
	}
}
