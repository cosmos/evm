package keeper

import (
	v2 "github.com/cosmos/evm/x/vm/migrations/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Migrator handles in-place store migrations between consensus versions.
type Migrator struct {
	keeper Keeper
}

// NewMigrator returns a new Migrator for the vm module.
func NewMigrator(k Keeper) Migrator {
	return Migrator{keeper: k}
}

// Migrate1to2 migrates from consensus version 1 to 2:
// rewrites the Params KV entry from the v0.2.x proto field layout to the
// v0.3.x layout (ChainConfig removed at field 5; remaining fields shifted).
func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	return v2.MigrateStore(ctx, m.keeper.storeKey)
}
