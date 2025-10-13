package keeper_test

import (
	"fmt"

	"github.com/cosmos/evm/x/precisebank/keeper"
	"github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (suite *KeeperIntegrationTestSuite) FundReserve(amt sdkmath.Int) {
	coins := sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom(), amt))
	err := suite.network.App.BankKeeper.MintCoins(suite.network.GetContext(), types.ModuleName, coins)
	suite.Require().NoError(err)
}

func (suite *KeeperIntegrationTestSuite) TestReserveBackingFractionalInvariant() {
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
			"valid - fractional balances, no remainder",
			func(ctx sdk.Context, k keeper.Keeper) {
				k.SetFractionalBalance(ctx, sdk.AccAddress{1}, types.ConversionFactor().QuoRaw(2))
				k.SetFractionalBalance(ctx, sdk.AccAddress{2}, types.ConversionFactor().QuoRaw(2))
				// 1 integer backs same amount fractional
				suite.FundReserve(sdkmath.NewInt(1))
			},
			false,
			"",
		},
		{
			"valid - fractional balances, with remainder",
			func(ctx sdk.Context, k keeper.Keeper) {
				k.SetFractionalBalance(ctx, sdk.AccAddress{1}, types.ConversionFactor().QuoRaw(2))
				k.SetRemainderAmount(ctx, types.ConversionFactor().QuoRaw(2))
				// 1 integer backs same amount fractional including remainder
				suite.FundReserve(sdkmath.NewInt(1))
			},
			false,
			"",
		},
		{
			"invalid - no fractional balances, non-zero remainder",
			func(ctx sdk.Context, k keeper.Keeper) {
				k.SetRemainderAmount(ctx, types.ConversionFactor().QuoRaw(2))
			},
			true,
			fmt.Sprintf("precisebank: module reserve backing total fractional balances invariant\n%s reserve balance 0 mismatches 500000000000 (fractional balances 0 + remainder 500000000000)\n\n",
				types.ExtendedCoinDenom()),
		},
		{
			"invalid - insufficient reserve backing",
			func(ctx sdk.Context, k keeper.Keeper) {
				amt := types.ConversionFactor().QuoRaw(2)

				// 0.5 int coins x 4
				k.SetFractionalBalance(ctx, sdk.AccAddress{1}, amt)
				k.SetFractionalBalance(ctx, sdk.AccAddress{2}, amt)
				k.SetFractionalBalance(ctx, sdk.AccAddress{3}, amt)
				k.SetRemainderAmount(ctx, amt)

				// Needs 2 to back 0.5 x 4
				suite.FundReserve(sdkmath.NewInt(1))
			},
			true,
			fmt.Sprintf("precisebank: module reserve backing total fractional balances invariant\n%s reserve balance 1000000000000 mismatches 2000000000000 (fractional balances 1500000000000 + remainder 500000000000)\n\n",
				types.ExtendedCoinDenom()),
		},
		{
			"invalid - excess reserve backing",
			func(ctx sdk.Context, k keeper.Keeper) {
				amt := types.ConversionFactor().QuoRaw(2)

				// 0.5 int coins x 4
				k.SetFractionalBalance(ctx, sdk.AccAddress{1}, amt)
				k.SetFractionalBalance(ctx, sdk.AccAddress{2}, amt)
				k.SetFractionalBalance(ctx, sdk.AccAddress{3}, amt)
				k.SetRemainderAmount(ctx, amt)

				// Needs 2 to back 0.5 x 4
				suite.FundReserve(sdkmath.NewInt(3))
			},
			true,
			fmt.Sprintf("precisebank: module reserve backing total fractional balances invariant\n%s reserve balance 3000000000000 mismatches 2000000000000 (fractional balances 1500000000000 + remainder 500000000000)\n\n",
				types.ExtendedCoinDenom()),
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset each time
			suite.SetupTest()

			tt.setupFn(suite.network.GetContext(), suite.network.App.PreciseBankKeeper)

			invariantFn := keeper.ReserveBacksFractionsInvariant(suite.network.App.PreciseBankKeeper)
			msg, broken := invariantFn(suite.network.GetContext())

			if tt.wantBroken {
				suite.Require().True(broken, "invariant should be broken but is not")
				suite.Require().Equal(tt.wantMsg, msg)
			} else {
				suite.Require().Falsef(broken, "invariant should not be broken but is: %s", msg)
			}
		})
	}
}

func (suite *KeeperIntegrationTestSuite) TestTotalSupplyInvariant() {
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
			"valid - mint fractional coins",
			func(ctx sdk.Context, k keeper.Keeper) {
				// Mint fractional coins equivalent to 1 uatom
				err := k.MintCoins(
					ctx,
					evmtypes.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.ExtendedCoinDenom(), types.ConversionFactor())),
				)
				suite.Require().NoError(err)
			},
			false,
			"",
		},
		{
			"valid - mint and send fractional coins",
			func(ctx sdk.Context, k keeper.Keeper) {
				// Mint fractional coins equivalent to 1 uatom
				err := k.MintCoins(
					ctx,
					evmtypes.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.ExtendedCoinDenom(), types.ConversionFactor())),
				)
				suite.Require().NoError(err)

				// Send 0.4 uatom worth to another account
				senderAddr := suite.network.App.AccountKeeper.GetModuleAddress(evmtypes.ModuleName)
				err = k.SendCoins(
					ctx,
					senderAddr,
					sdk.AccAddress{1},
					sdk.NewCoins(sdk.NewCoin(types.ExtendedCoinDenom(), types.ConversionFactor().QuoRaw(10).MulRaw(4))),
				)
				suite.Require().NoError(err)
			},
			false,
			"",
		},
		{
			"valid - mint, send, and burn operations",
			func(ctx sdk.Context, k keeper.Keeper) {
				// Mint fractional coins equivalent to 1 uatom
				err := k.MintCoins(
					ctx,
					evmtypes.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.ExtendedCoinDenom(), types.ConversionFactor())),
				)
				suite.Require().NoError(err)

				// Send 0.4 uatom worth to another account
				senderAddr := suite.network.App.AccountKeeper.GetModuleAddress(evmtypes.ModuleName)
				err = k.SendCoins(
					ctx,
					senderAddr,
					sdk.AccAddress{1},
					sdk.NewCoins(sdk.NewCoin(types.ExtendedCoinDenom(), types.ConversionFactor().QuoRaw(10).MulRaw(4))),
				)
				suite.Require().NoError(err)

				// Burn fractional coins equivalent to 0.2 uatom
				err = k.BurnCoins(
					ctx,
					evmtypes.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.ExtendedCoinDenom(), types.ConversionFactor().QuoRaw(10).MulRaw(2))),
				)
				suite.Require().NoError(err)
			},
			false,
			"",
		},
		{
			"invalid - mismatch due to incorrect balance manipulation",
			func(ctx sdk.Context, k keeper.Keeper) {
				// Mint fractional coins equivalent to 1000 aatom
				err := k.MintCoins(
					ctx,
					evmtypes.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.ExtendedCoinDenom(), sdkmath.NewInt(1000))),
				)
				suite.Require().NoError(err)

				// Directly modify fractional balance to create mismatch
				// (this should never happen in practice)
				k.SetFractionalBalance(
					ctx,
					sdk.AccAddress{1},
					types.ConversionFactor().QuoRaw(10).MulRaw(7),
				)
			},
			true,
			"precisebank: total-supply invariant\ntotal supply 200003000001700000000000 does not match integer total supply 200003000001000000000000\n",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset each time
			suite.SetupTest()

			suite.network.App.BankKeeper.IterateAllBalances(
				suite.network.GetContext(),
				func(address sdk.AccAddress, coin sdk.Coin) (stop bool) {
					return false
				},
			)

			tt.setupFn(suite.network.GetContext(), suite.network.App.PreciseBankKeeper)

			invariantFn := keeper.TotalSupplyInvariant(suite.network.App.PreciseBankKeeper)
			msg, broken := invariantFn(suite.network.GetContext())

			if tt.wantBroken {
				suite.Require().True(broken, "invariant should be broken but is not")
				suite.Require().Equal(tt.wantMsg, msg)
			} else {
				suite.Require().False(broken, "invariant should not be broken but is")
			}
		})
	}
}
