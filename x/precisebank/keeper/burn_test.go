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
	td := newMockedTestData(t)

	// panic tests for invalid inputs
	tests := []struct {
		name            string
		recipientModule string
		setupFn         func(td testData)
		burnAmount      sdk.Coins
		wantPanic       string
	}{
		{
			"invalid module",
			"notamodule",
			func(td testData) {
				// Make module not found
				td.ak.EXPECT().
					GetModuleAccount(td.ctx, "notamodule").
					Return(nil).
					Once()
			},
			cs(c(types.IntegerCoinDenom(), 1000)),
			"module account notamodule does not exist: unknown address",
		},
		{
			"no permission",
			burnerModuleName,
			func(td testData) {
				td.ak.EXPECT().
					GetModuleAccount(td.ctx, burnerModuleName).
					Return(authtypes.NewModuleAccount(
						authtypes.NewBaseAccountWithAddress(sdk.AccAddress{1}),
						burnerModuleName,
						// no burn permission
					)).
					Once()
			},
			cs(c(types.IntegerCoinDenom(), 1000)),
			fmt.Sprintf("module account %s does not have permissions to burn tokens: unauthorized", burnerModuleName),
		},
		{
			"has burn permission",
			burnerModuleName,
			func(td testData) {
				td.ak.EXPECT().
					GetModuleAccount(td.ctx, burnerModuleName).
					Return(authtypes.NewModuleAccount(
						authtypes.NewBaseAccountWithAddress(sdk.AccAddress{1}),
						burnerModuleName,
						// includes burner permission
						authtypes.Burner,
					)).
					Once()

				// Will call x/bank BurnCoins coins
				td.bk.EXPECT().
					BurnCoins(td.ctx, burnerModuleName, cs(c(types.IntegerCoinDenom(), 1000))).
					Return(nil).
					Once()
			},
			cs(c(types.IntegerCoinDenom(), 1000)),
			"",
		},
		{
			"disallow burning from x/precisebank",
			types.ModuleName,
			func(td testData) {
				// No mock setup needed since this is checked before module
				// account checks
			},
			cs(c(types.IntegerCoinDenom(), 1000)),
			"module account precisebank cannot be burned from: unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFn(td)

			if tt.wantPanic != "" {
				require.PanicsWithError(t, tt.wantPanic, func() {
					_ = td.keeper.BurnCoins(td.ctx, tt.recipientModule, tt.burnAmount)
				})
				return
			}

			require.NotPanics(t, func() {
				// Not testing errors, only panics for this test
				_ = td.keeper.BurnCoins(td.ctx, tt.recipientModule, tt.burnAmount)
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
