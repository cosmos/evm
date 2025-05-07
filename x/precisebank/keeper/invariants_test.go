package keeper_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/precisebank/keeper"
	"github.com/cosmos/evm/x/precisebank/types"
	"github.com/cosmos/evm/x/precisebank/types/mocks"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestBalancedFractionalTotalInvariant(t *testing.T) {
	tests := []struct {
		name       string
		setupFn    func(ctx sdk.Context, k keeper.Keeper)
		wantBroken bool
		wantMsg    string
	}{
		{
			"valid - empty state",
			func(_ sdk.Context, _ keeper.Keeper) {},
			false,
			"",
		},
		{
			"valid - balances, 0 remainder",
			func(ctx sdk.Context, k keeper.Keeper) {
				k.SetFractionalBalance(ctx, sdk.AccAddress{1}, types.ConversionFactor().QuoRaw(2))
				k.SetFractionalBalance(ctx, sdk.AccAddress{2}, types.ConversionFactor().QuoRaw(2))
			},
			false,
			"",
		},
		{
			"valid - balances, non-zero remainder",
			func(ctx sdk.Context, k keeper.Keeper) {
				k.SetFractionalBalance(ctx, sdk.AccAddress{1}, types.ConversionFactor().QuoRaw(2))
				k.SetFractionalBalance(ctx, sdk.AccAddress{2}, types.ConversionFactor().QuoRaw(2).SubRaw(1))

				k.SetRemainderAmount(ctx, sdkmath.OneInt())
			},
			false,
			"",
		},
		{
			"invalid - balances, 0 remainder",
			func(ctx sdk.Context, k keeper.Keeper) {
				k.SetFractionalBalance(ctx, sdk.AccAddress{1}, types.ConversionFactor().QuoRaw(2))
				k.SetFractionalBalance(ctx, sdk.AccAddress{2}, types.ConversionFactor().QuoRaw(2).SubRaw(1))
			},
			true,
			"precisebank: balance-remainder-total invariant\n(sum(FractionalBalances) + remainder) % conversionFactor should be 0 but got 999999999999\n",
		},
		{
			"invalid - invalid balances, non-zero (insufficient) remainder",
			func(ctx sdk.Context, k keeper.Keeper) {
				k.SetFractionalBalance(ctx, sdk.AccAddress{1}, types.ConversionFactor().QuoRaw(2))
				k.SetFractionalBalance(ctx, sdk.AccAddress{2}, types.ConversionFactor().QuoRaw(2).SubRaw(2))
				k.SetRemainderAmount(ctx, sdkmath.OneInt())
			},
			true,
			"precisebank: balance-remainder-total invariant\n(sum(FractionalBalances) + remainder) % conversionFactor should be 0 but got 999999999999\n",
		},
		{
			"invalid - invalid balances, non-zero (excess) remainder",
			func(ctx sdk.Context, k keeper.Keeper) {
				k.SetFractionalBalance(ctx, sdk.AccAddress{1}, types.ConversionFactor().QuoRaw(2))
				k.SetFractionalBalance(ctx, sdk.AccAddress{2}, types.ConversionFactor().QuoRaw(2).SubRaw(2))
				k.SetRemainderAmount(ctx, sdkmath.NewInt(5))
			},
			true,
			"precisebank: balance-remainder-total invariant\n(sum(FractionalBalances) + remainder) % conversionFactor should be 0 but got 3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset each time
			td := newMockedTestData(t)

			tt.setupFn(td.ctx, td.keeper)

			invariantFn := keeper.BalancedFractionalTotalInvariant(td.keeper)
			msg, broken := invariantFn(td.ctx)

			if tt.wantBroken {
				require.True(t, broken, "invariant should be broken but is not")
				require.Equal(t, tt.wantMsg, msg)
			} else {
				require.False(t, broken, "invariant should not be broken but is")
			}
		})
	}
}

func TestValidFractionalAmountsInvariant(t *testing.T) {
	tests := []struct {
		name       string
		setupFn    func(ctx sdk.Context, k keeper.Keeper, storeKey storetypes.StoreKey)
		wantBroken bool
		wantMsg    string
	}{
		{
			"valid - empty state",
			func(_ sdk.Context, _ keeper.Keeper, _ storetypes.StoreKey) {},
			false,
			"",
		},
		{
			"valid - valid balances",
			func(ctx sdk.Context, k keeper.Keeper, _ storetypes.StoreKey) {
				k.SetFractionalBalance(ctx, sdk.AccAddress{1}, types.ConversionFactor().QuoRaw(2))
				k.SetFractionalBalance(ctx, sdk.AccAddress{2}, types.ConversionFactor().QuoRaw(2))
			},
			false,
			"",
		},
		{
			"invalid - exceeds max balance",
			func(ctx sdk.Context, _ keeper.Keeper, storeKey storetypes.StoreKey) {
				// Requires manual store manipulation so it is unlikely to have
				// invalid state in practice. SetFractionalBalance will validate
				// before setting.
				addr := sdk.AccAddress{1}
				amount := types.ConversionFactor()

				store := prefix.NewStore(ctx.KVStore(storeKey), types.FractionalBalancePrefix)

				amountBytes, err := amount.Marshal()
				require.NoError(t, err)

				store.Set(types.FractionalBalanceKey(addr), amountBytes)
			},
			true,
			"precisebank: valid-fractional-balances invariant\namount of invalid fractional balances found 1\n\tcosmos1qyfkm2y3 has an invalid fractional amount of 1000000000000\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset each time
			td := newMockedTestData(t)

			tt.setupFn(td.ctx, td.keeper, td.storeKey)

			invariantFn := keeper.ValidFractionalAmountsInvariant(td.keeper)
			msg, broken := invariantFn(td.ctx)

			if tt.wantBroken {
				require.True(t, broken, "invariant should be broken but is not")
				require.Equal(t, tt.wantMsg, msg)
			} else {
				require.False(t, broken, "invariant should not be broken but is")
			}
		})
	}
}

func TestFractionalDenomNotInBankInvariant(t *testing.T) {
	tests := []struct {
		name       string
		setupFn    func(ctx sdk.Context, bk *mocks.MockBankKeeper)
		wantBroken bool
		wantMsg    string
	}{
		{
			"valid - integer denom (uatom) supply",
			func(ctx sdk.Context, bk *mocks.MockBankKeeper) {
				// No fractional balance in x/bank
				// This also enforces there is no GetSupply() call for IntegerCoinDenom / uatom
				bk.EXPECT().
					GetSupply(ctx, types.ExtendedCoinDenom()).
					Return(sdk.NewCoin(types.ExtendedCoinDenom(), sdkmath.ZeroInt())).
					Once()
			},
			false,
			"",
		},
		{
			"invalid - x/bank contains fractional denom (aatom)",
			func(ctx sdk.Context, bk *mocks.MockBankKeeper) {
				bk.EXPECT().
					GetSupply(ctx, types.ExtendedCoinDenom()).
					Return(sdk.NewCoin(types.ExtendedCoinDenom(), sdkmath.NewInt(1000))).
					Once()
			},
			true,
			fmt.Sprintf("precisebank: fractional-denom-not-in-bank invariant\nx/bank should not hold any %s but has supply of 1000%s\n",
				types.ExtendedCoinDenom(), types.ExtendedCoinDenom()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset each time
			td := newMockedTestData(t)

			tt.setupFn(td.ctx, td.bk)

			invariantFn := keeper.FractionalDenomNotInBankInvariant(td.keeper)
			msg, broken := invariantFn(td.ctx)

			if tt.wantBroken {
				require.True(t, broken, "invariant should be broken but is not")
				require.Equal(t, tt.wantMsg, msg)
			} else {
				require.False(t, broken, "invariant should not be broken but is")
			}
		})
	}
}
