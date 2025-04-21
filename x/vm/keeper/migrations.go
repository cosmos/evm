package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	v9 "github.com/cosmos/evm/x/vm/migrations/v9"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	keeper Keeper
}

// NewMigrator returns a new Migrator.
func NewMigrator(keeper Keeper) Migrator {
	return Migrator{
		keeper: keeper,
	}
}

// Migrate4to5 migrates the store from consensus version 4 to 5
func (m Migrator) Migrate8to9(ctx sdk.Context) error {
	return v9.MigrateStore(ctx, m.keeper.storeKey, m.keeper.cdc)
}
