package keeper

import (
	"github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	keeper *Keeper
}

// NewMigrator returns a new Migrator.
func NewMigrator(keeper *Keeper) Migrator {
	return Migrator{keeper: keeper}
}

// Migrate9to10 migrates the x/vm module state from the consensus
// version 9 to version 10. Specifically, it deploys a set of predefined
// preinstall contracts with a fixed addresses and code.
func (m Migrator) Migrate9to10(ctx sdk.Context) error {
	return m.keeper.AddPreinstalls(ctx, types.DefaultPreinstalls)
}
