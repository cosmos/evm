package keeper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	testconfig "github.com/cosmos/evm/testutil/config"
	"github.com/cosmos/evm/x/precisebank/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

func TestKeeper_GetBalance(t *testing.T) {
	tk := newMockedTestData(t)

	integerDenom := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.GetDenom()
	extendedDenom := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.GetExtendedDenom()
	extendedDecimals := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.ExtendedDecimals

	tests := []struct {
		name      string
		giveDenom string // queried denom for balance

		giveBankBal       sdk.Coins   // mocked bank balance for giveAddr
		giveFractionalBal sdkmath.Int // stored fractional balance for giveAddr

		wantBal sdk.Coin
	}{
		{
			"extended denom - no fractional balance",
			extendedDenom,
			// queried bank balance in uatom when querying for aatom
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(1000))),
			sdkmath.ZeroInt(),
			// integer + fractional
			sdk.NewCoin(extendedDenom, sdkmath.NewInt(1000_000_000_000_000)),
		},
		{
			"extended denom - with fractional balance",
			extendedDenom,
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(1000))),
			sdkmath.NewInt(100),
			// integer + fractional
			sdk.NewCoin(extendedDenom, sdkmath.NewInt(1000_000_000_000_100)),
		},
		{
			"extended denom - only fractional balance",
			extendedDenom,
			// no coins in bank, only fractional balance
			sdk.NewCoins(),
			sdkmath.NewInt(100),
			sdk.NewCoin(extendedDenom, sdkmath.NewInt(100)),
		},
		{
			"extended denom - max fractional balance",
			extendedDenom,
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(1000))),
			types.ConversionFactor(extendedDecimals).SubRaw(1),
			// integer + fractional
			sdk.NewCoin(extendedDenom, sdkmath.NewInt(1000_999_999_999_999)),
		},
		{
			"non-extended denom - uatom returns uatom",
			integerDenom,
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(1000))),
			sdkmath.ZeroInt(),
			sdk.NewCoin(integerDenom, sdkmath.NewInt(1000)),
		},
		{
			"non-extended denom - unaffected by fractional balance",
			integerDenom,
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(1000))),
			sdkmath.NewInt(100),
			sdk.NewCoin(integerDenom, sdkmath.NewInt(1000)),
		},
		{
			"unrelated denom - no fractional",
			"busd",
			sdk.NewCoins(sdk.NewCoin("busd", sdkmath.NewInt(1000))),
			sdkmath.ZeroInt(),
			sdk.NewCoin("busd", sdkmath.NewInt(1000)),
		},
		{
			"unrelated denom - unaffected by fractional balance",
			"busd",
			sdk.NewCoins(sdk.NewCoin("busd", sdkmath.NewInt(1000))),
			sdkmath.NewInt(100),
			sdk.NewCoin("busd", sdkmath.NewInt(1000)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := sdk.AccAddress([]byte("test-address"))

			// Set fractional balance in store before query
			tk.keeper.SetFractionalBalance(tk.ctx, addr, tt.giveFractionalBal)

			// Checks address if its a reserve denom
			if tt.giveDenom == extendedDenom {
				tk.ak.EXPECT().GetModuleAddress(types.ModuleName).
					Return(authtypes.NewModuleAddress(types.ModuleName)).
					Once()
			}

			if tt.giveDenom == extendedDenom {
				// No balance pass through
				tk.bk.EXPECT().
					GetBalance(tk.ctx, addr, integerDenom).
					RunAndReturn(func(_ context.Context, _ sdk.AccAddress, _ string) sdk.Coin {
						amt := tt.giveBankBal.AmountOf(integerDenom)
						return sdk.NewCoin(integerDenom, amt)
					}).
					Once()
			} else {
				// Pass through to x/bank for denoms except ExtendedCoinDenom
				tk.bk.EXPECT().
					GetBalance(tk.ctx, addr, tt.giveDenom).
					RunAndReturn(func(ctx context.Context, aa sdk.AccAddress, s string) sdk.Coin {
						require.Equal(t, s, tt.giveDenom, "unexpected denom passed to x/bank.GetBalance")

						return sdk.NewCoin(tt.giveDenom, tt.giveBankBal.AmountOf(s))
					}).
					Once()
			}

			bal := tk.keeper.GetBalance(tk.ctx, addr, tt.giveDenom)
			require.Equal(t, tt.wantBal, bal)
		})
	}
}

func TestKeeper_SpendableCoin(t *testing.T) {
	integerDenom := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.GetDenom()
	extendedDenom := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.GetExtendedDenom()
	extendedDecimals := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.ExtendedDecimals

	tests := []struct {
		name      string
		giveDenom string // queried denom for balance

		giveBankBal       sdk.Coins   // mocked bank balance for giveAddr
		giveFractionalBal sdkmath.Int // stored fractional balance for giveAddr

		wantBal sdk.Coin
	}{
		{
			"extended denom - no fractional balance",
			extendedDenom,
			// queried bank balance in uatom when querying for aatom
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(1000))),
			sdkmath.ZeroInt(),
			// integer + fractional
			sdk.NewCoin(extendedDenom, sdkmath.NewInt(1000_000_000_000_000)),
		},
		{
			"extended denom - with fractional balance",
			extendedDenom,
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(1000))),
			sdkmath.NewInt(100),
			// integer + fractional
			sdk.NewCoin(extendedDenom, sdkmath.NewInt(1000_000_000_000_100)),
		},
		{
			"extended denom - only fractional balance",
			extendedDenom,
			// no coins in bank, only fractional balance
			sdk.NewCoins(),
			sdkmath.NewInt(100),
			sdk.NewCoin(extendedDenom, sdkmath.NewInt(100)),
		},
		{
			"extended denom - max fractional balance",
			extendedDenom,
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(1000))),
			types.ConversionFactor(extendedDecimals).SubRaw(1),
			// integer + fractional
			sdk.NewCoin(extendedDenom, sdkmath.NewInt(1000_999_999_999_999)),
		},
		{
			"non-extended denom - uatom returns uatom",
			integerDenom,
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(1000))),
			sdkmath.ZeroInt(),
			sdk.NewCoin(integerDenom, sdkmath.NewInt(1000)),
		},
		{
			"non-extended denom - unaffected by fractional balance",
			integerDenom,
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(1000))),
			sdkmath.NewInt(100),
			sdk.NewCoin(integerDenom, sdkmath.NewInt(1000)),
		},
		{
			"unrelated denom - no fractional",
			"busd",
			sdk.NewCoins(sdk.NewCoin("busd", sdkmath.NewInt(1000))),
			sdkmath.ZeroInt(),
			sdk.NewCoin("busd", sdkmath.NewInt(1000)),
		},
		{
			"unrelated denom - unaffected by fractional balance",
			"busd",
			sdk.NewCoins(sdk.NewCoin("busd", sdkmath.NewInt(1000))),
			sdkmath.NewInt(100),
			sdk.NewCoin("busd", sdkmath.NewInt(1000)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := newMockedTestData(t)
			addr := sdk.AccAddress([]byte("test-address"))

			// Set fractional balance in store before query
			tk.keeper.SetFractionalBalance(tk.ctx, addr, tt.giveFractionalBal)

			// If its a reserve denom, module address is checked
			if tt.giveDenom == extendedDenom {
				tk.ak.EXPECT().GetModuleAddress(types.ModuleName).
					Return(authtypes.NewModuleAddress(types.ModuleName)).
					Once()
			}

			if tt.giveDenom == extendedDenom {
				// No balance pass through
				tk.bk.EXPECT().
					SpendableCoin(tk.ctx, addr, integerDenom).
					RunAndReturn(func(_ context.Context, _ sdk.AccAddress, _ string) sdk.Coin {
						amt := tt.giveBankBal.AmountOf(integerDenom)
						return sdk.NewCoin(integerDenom, amt)
					}).
					Once()
			} else {
				// Pass through to x/bank for denoms except ExtendedCoinDenom
				tk.bk.EXPECT().
					SpendableCoin(tk.ctx, addr, tt.giveDenom).
					RunAndReturn(func(ctx context.Context, aa sdk.AccAddress, s string) sdk.Coin {
						require.Equal(t, s, tt.giveDenom, "unexpected denom passed to x/bank.GetBalance")

						return sdk.NewCoin(tt.giveDenom, tt.giveBankBal.AmountOf(s))
					}).
					Once()
			}

			bal := tk.keeper.SpendableCoin(tk.ctx, addr, tt.giveDenom)
			require.Equal(t, tt.wantBal, bal)
		})
	}
}

func TestHiddenReserve(t *testing.T) {
	// Reserve balances should not be shown to consumers of x/precisebank, as it
	// represents the fractional balances of accounts.

	tk := newMockedTestData(t)

	moduleAddr := authtypes.NewModuleAddress(types.ModuleName)

	integerDenom := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.GetDenom()
	extendedDenom := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.GetExtendedDenom()

	// No mock bankkeeper expectations, which means the zero coin is returned
	// directly for reserve address. So the mock bankkeeper doesn't need to have
	// a handler for getting underlying balance.

	tests := []struct {
		name            string
		denom           string
		expectedBalance sdk.Coin
	}{
		{
			"aatom",
			extendedDenom,
			sdk.NewCoin(extendedDenom, sdkmath.ZeroInt()),
		},
		{
			"uatom",
			integerDenom,
			sdk.NewCoin(integerDenom, sdkmath.NewInt(1)),
		},
		{
			"unrelated denom",
			"cat",
			sdk.NewCoin("cat", sdkmath.ZeroInt()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 2 calls for GetBalance and SpendableCoin, only for reserve coins
			if tt.denom == extendedDenom {
				tk.ak.EXPECT().GetModuleAddress(types.ModuleName).
					Return(moduleAddr).
					Twice()
			} else {
				// Passthrough to x/bank for non-reserve denoms
				tk.bk.EXPECT().
					GetBalance(tk.ctx, moduleAddr, tt.denom).
					Return(sdk.NewCoin(tt.denom, sdkmath.ZeroInt())).
					Once()

				tk.bk.EXPECT().
					SpendableCoin(tk.ctx, moduleAddr, tt.denom).
					Return(sdk.NewCoin(tt.denom, sdkmath.ZeroInt())).
					Once()
			}

			// GetBalance should return zero balance for reserve address
			coin := tk.keeper.GetBalance(tk.ctx, moduleAddr, tt.denom)
			require.Equal(t, tt.denom, coin.Denom)
			require.Equal(t, sdkmath.ZeroInt(), coin.Amount)

			// SpendableCoin should return zero balance for reserve address
			spendableCoin := tk.keeper.SpendableCoin(tk.ctx, moduleAddr, tt.denom)
			require.Equal(t, tt.denom, spendableCoin.Denom)
			require.Equal(t, sdkmath.ZeroInt(), spendableCoin.Amount)
		})
	}
}
