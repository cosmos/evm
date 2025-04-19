package keeper_test

import (
	"github.com/cosmos/evm/x/precisebank/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

func (suite *KeeperIntegrationTestSuite) TestKeeper_SpendableCoin() {
	tests := []struct {
		name      string
		giveDenom string // queried denom for balance

		giveBankBal       sdk.Coins   // full balance
		giveFractionalBal sdkmath.Int // stored fractional balance for giveAddr

		wantSpendableBal sdk.Coin
	}{
		{
			"extended denom, no fractional - locked coins",
			types.ExtendedCoinDenom,
			// queried bank balance in uatom when querying for aatom
			sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom, sdkmath.NewInt(1000))),
			sdkmath.ZeroInt(),
			sdk.NewCoin(
				types.ExtendedCoinDenom,
				types.ConversionFactor().MulRaw(1000),
			),
		},
		{
			"extended denom, with fractional - locked coins",
			types.ExtendedCoinDenom,
			// queried bank balance in uatom when querying for aatom
			sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom, sdkmath.NewInt(1000))),
			sdkmath.NewInt(5000),
			sdk.NewCoin(
				types.ExtendedCoinDenom,
				types.ConversionFactor().MulRaw(1000).AddRaw(5000),
			),
		},
		{
			"non-extended denom - uatom returns uatom",
			types.IntegerCoinDenom,
			sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom, sdkmath.NewInt(1000))),
			sdkmath.ZeroInt(),
			sdk.NewCoin(types.IntegerCoinDenom, sdkmath.NewInt(1000)),
		},
		{
			"non-extended denom, with fractional - uatom returns uatom",
			types.IntegerCoinDenom,
			sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom, sdkmath.NewInt(1000))),
			// does not affect balance
			sdkmath.NewInt(100),
			sdk.NewCoin(types.IntegerCoinDenom, sdkmath.NewInt(1000)),
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.SetupTest()

			addr := sdk.AccAddress([]byte("test-address"))

			suite.MintToAccount(addr, tt.giveBankBal)

			// Set fractional balance in store before query
			suite.network.App.PreciseBankKeeper.SetFractionalBalance(suite.network.GetContext(), addr, tt.giveFractionalBal)

			spendableCoins := suite.network.App.PreciseBankKeeper.SpendableCoin(suite.network.GetContext(), addr, tt.giveDenom)

			suite.Require().Equalf(
				tt.wantSpendableBal,
				spendableCoins,
				"expected spendable coins of denom %s",
				tt.giveDenom,
			)
		})
	}
}

func (suite *KeeperIntegrationTestSuite) TestKeeper_HiddenReserve() {
	// Reserve balances should not be shown to consumers of x/precisebank, as it
	// represents the fractional balances of accounts.

	moduleAddr := authtypes.NewModuleAddress(types.ModuleName)
	addr1 := sdk.AccAddress{1}

	// Make the reserve hold a non-zero balance
	// Mint fractional coins to an account, which should cause a mint of 1
	// integer coin to the reserve to back it.
	extCoin := sdk.NewCoin(types.ExtendedCoinDenom, types.ConversionFactor().AddRaw(1000))
	unrelatedCoin := sdk.NewCoin("unrelated", sdkmath.NewInt(1000))
	suite.MintToAccount(
		addr1,
		sdk.NewCoins(
			extCoin,
			unrelatedCoin,
		),
	)

	// Check underlying x/bank balance for reserve
	reserveIntCoin := suite.network.App.BankKeeper.GetBalance(suite.network.GetContext(), moduleAddr, types.IntegerCoinDenom)
	suite.Require().Equal(
		sdkmath.NewInt(1),
		reserveIntCoin.Amount,
		"reserve should hold 1 integer coin",
	)

	tests := []struct {
		name       string
		giveAddr   sdk.AccAddress
		giveDenom  string
		wantAmount sdkmath.Int
	}{
		{
			"reserve account - hidden extended denom",
			moduleAddr,
			types.ExtendedCoinDenom,
			sdkmath.ZeroInt(),
		},
		{
			"reserve account - visible integer denom",
			moduleAddr,
			types.IntegerCoinDenom,
			sdkmath.OneInt(),
		},
		{
			"user account - visible extended denom",
			addr1,
			types.ExtendedCoinDenom,
			extCoin.Amount,
		},
		{
			"user account - visible integer denom",
			addr1,
			types.IntegerCoinDenom,
			extCoin.Amount.Quo(types.ConversionFactor()),
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			coin := suite.network.App.PreciseBankKeeper.GetBalance(suite.network.GetContext(), tt.giveAddr, tt.giveDenom)
			suite.Require().Equal(tt.wantAmount.Int64(), coin.Amount.Int64())

			spendableCoin := suite.network.App.PreciseBankKeeper.SpendableCoin(suite.network.GetContext(), tt.giveAddr, tt.giveDenom)
			suite.Require().Equal(tt.wantAmount.Int64(), spendableCoin.Amount.Int64())
		})
	}
}
