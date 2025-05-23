package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/x/vm/types"
)

// Event Hooks
// These can be utilized to customize evm transaction processing.

var _ types.MigrateAccountHooks = MultiMigrateAccountHooks{}

// MultiMigrateAccountHooks TODO
type MultiMigrateAccountHooks []types.MigrateAccountHooks

// NewMultiMigrateAccountHooks TODO
func NewMultiMigrateAccountHooks(hooks ...types.MigrateAccountHooks) MultiMigrateAccountHooks {
	return hooks
}

func (m MultiMigrateAccountHooks) BeforeAll(ctx sdk.Context, originalAddress sdk.AccAddress, newAddress sdk.AccAddress) error {
	for i := range m {
		if err := m[i].BeforeAll(ctx, originalAddress, newAddress); err != nil {
			return err
		}
	}
	return nil
}

func (m MultiMigrateAccountHooks) AfterMigrateDelegations(ctx sdk.Context, originalAddress sdk.AccAddress, newAddress sdk.AccAddress) error {
	for i := range m {
		if err := m[i].BeforeAll(ctx, originalAddress, newAddress); err != nil {
			return err
		}
	}
	return nil
}

func (m MultiMigrateAccountHooks) AfterMigrateBankTokens(ctx sdk.Context, originalAddress sdk.AccAddress, newAddress sdk.AccAddress) error {
	for i := range m {
		if err := m[i].BeforeAll(ctx, originalAddress, newAddress); err != nil {
			return err
		}
	}
	return nil
}

func (m MultiMigrateAccountHooks) AfterMigrateFeeGrants(ctx sdk.Context, originalAddress sdk.AccAddress, newAddress sdk.AccAddress) error {
	for i := range m {
		if err := m[i].BeforeAll(ctx, originalAddress, newAddress); err != nil {
			return err
		}
	}
	return nil
}

func (m MultiMigrateAccountHooks) AfterAll(ctx sdk.Context, originalAddress sdk.AccAddress, newAddress sdk.AccAddress) error {
	for i := range m {
		if err := m[i].BeforeAll(ctx, originalAddress, newAddress); err != nil {
			return err
		}
	}
	return nil
}
