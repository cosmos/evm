package keeper_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/precisebank/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// Testing module name for mocked GetModuleAccount()
const burnerModuleName = "burner-module"

func TestBurnCoins_PanicValidations(t *testing.T) {
	// panic tests for invalid inputs
	tests := []struct {
		name            string
		recipientModule string
		setupFn         func(td testData) sdk.Coins // Return coins after EVM setup
		wantPanic       string
	}{
		{
			"invalid module",
			"notamodule",
			func(td testData) sdk.Coins {
				// Make module not found - this will panic before GetModuleAddress is called
				td.ak.EXPECT().
					GetModuleAccount(td.ctx, "notamodule").
					Return(nil).
					Once()
				return cs(c(types.ExtendedCoinDenom(), 1000))
			},
			"module account notamodule does not exist: unknown address",
		},
		{
			"no permission",
			burnerModuleName,
			func(td testData) sdk.Coins {
				moduleAddr := sdk.AccAddress{1}
				// This will panic before GetModuleAddress is called
				td.ak.EXPECT().
					GetModuleAccount(td.ctx, burnerModuleName).
					Return(authtypes.NewModuleAccount(
						authtypes.NewBaseAccountWithAddress(moduleAddr),
						burnerModuleName,
						// no burn permission
					)).
					Once()
				return cs(c(types.ExtendedCoinDenom(), 1000))
			},
			fmt.Sprintf("module account %s does not have permissions to burn tokens: unauthorized", burnerModuleName),
		},
		{
			"has burn permission",
			burnerModuleName,
			func(td testData) sdk.Coins {
				moduleAddr := sdk.AccAddress{1}
				// This case succeeds and calls burnExtendedCoin, which calls GetModuleAddress once
				td.ak.EXPECT().
					GetModuleAddress(burnerModuleName).
					Return(moduleAddr).
					Once()

				td.ak.EXPECT().
					GetModuleAccount(td.ctx, burnerModuleName).
					Return(authtypes.NewModuleAccount(
						authtypes.NewBaseAccountWithAddress(moduleAddr),
						burnerModuleName,
						// includes burner permission
						authtypes.Burner,
					)).
					Once()

				coins := cs(c(types.ExtendedCoinDenom(), 1000))
				// Will call burnExtendedCoin which calls GetModuleAddress
				// Also need to borrow 1 integer coin to cover the fractional burn
				borrowCoin := cs(c(types.IntegerCoinDenom(), 1))
				td.bk.EXPECT().
					SendCoinsFromModuleToModule(td.ctx, burnerModuleName, types.ModuleName, borrowCoin).
					Return(nil).
					Once()
				return coins
			},
			"",
		},
		{
			"disallow burning from x/precisebank",
			types.ModuleName,
			func(td testData) sdk.Coins {
				// No expectations needed - this should panic before any module address calls
				return cs(c(types.ExtendedCoinDenom(), 1000))
			},
			"module account precisebank cannot be burned from: unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newMockedTestData(t)
			burnAmount := tt.setupFn(td)

			if tt.wantPanic != "" {
				require.PanicsWithError(t, tt.wantPanic, func() {
					_ = td.keeper.BurnCoins(td.ctx, tt.recipientModule, burnAmount)
				})
				return
			}

			require.NotPanics(t, func() {
				// Not testing errors, only panics for this test
				_ = td.keeper.BurnCoins(td.ctx, tt.recipientModule, burnAmount)
			})
		})
	}
}

func TestBurnCoins_Errors(t *testing.T) {
	// returned errors, not panics

	tests := []struct {
		name            string
		recipientModule string
		setupFn         func(td testData)
		burnAmount      sdk.Coins
		wantError       string
	}{
		{
			"invalid coins",
			burnerModuleName,
			func(td testData) {
				// Valid module account burner
				td.ak.EXPECT().
					GetModuleAccount(td.ctx, burnerModuleName).
					Return(authtypes.NewModuleAccount(
						authtypes.NewBaseAccountWithAddress(sdk.AccAddress{1}),
						burnerModuleName,
						// includes burner permission
						authtypes.Burner,
					)).
					Once()
			},
			sdk.Coins{sdk.Coin{
				Denom:  types.IntegerCoinDenom(),
				Amount: sdkmath.NewInt(-1000),
			}},
			fmt.Sprintf("-1000%s: invalid coins", types.IntegerCoinDenom()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newMockedTestData(t)
			tt.setupFn(td)

			require.NotPanics(t, func() {
				err := td.keeper.BurnCoins(td.ctx, tt.recipientModule, tt.burnAmount)

				if tt.wantError != "" {
					require.Error(t, err)
					require.EqualError(t, err, tt.wantError)
					return
				}

				require.NoError(t, err)
			})
		})
	}
}
