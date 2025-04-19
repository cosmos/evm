package keeper_test

import (
	"context"
	"strconv"

	"github.com/cosmos/evm/x/precisebank/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

func (suite *KeeperIntegrationTestSuite) TestQueryTotalFractionalBalance() {
	testCases := []struct {
		name         string
		giveBalances []sdkmath.Int
	}{
		{
			"empty",
			[]sdkmath.Int{},
		},
		{
			"min amount",
			[]sdkmath.Int{
				types.ConversionFactor().QuoRaw(2),
				types.ConversionFactor().QuoRaw(2),
			},
		},
		{
			"exceeds conversion factor",
			[]sdkmath.Int{
				// 4 accounts * 0.5 == 2
				types.ConversionFactor().QuoRaw(2),
				types.ConversionFactor().QuoRaw(2),
				types.ConversionFactor().QuoRaw(2),
				types.ConversionFactor().QuoRaw(2),
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			total := sdk.NewCoin(types.ExtendedCoinDenom, sdkmath.ZeroInt())
			for i, balance := range tc.giveBalances {
				addr := sdk.AccAddress([]byte(strconv.Itoa(i)))
				suite.network.App.PreciseBankKeeper.SetFractionalBalance(suite.network.GetContext(), addr, balance)

				total.Amount = total.Amount.Add(balance)
			}

			res, err := suite.network.GetPreciseBankClient().TotalFractionalBalances(
				context.Background(),
				&types.QueryTotalFractionalBalancesRequest{},
			)
			suite.Require().NoError(err)

			suite.Require().Equal(total, res.Total)
		})
	}
}

func (suite *KeeperIntegrationTestSuite) TestQueryRemainder() {
	res, err := suite.network.GetPreciseBankClient().Remainder(
		context.Background(),
		&types.QueryRemainderRequest{},
	)
	suite.Require().NoError(err)

	expRemainder := sdk.NewCoin(types.ExtendedCoinDenom, sdkmath.ZeroInt())
	suite.Require().Equal(expRemainder, res.Remainder)

	// Mint fractional coins to create non-zero remainder

	pbk := suite.network.App.PreciseBankKeeper

	coin := sdk.NewCoin(types.ExtendedCoinDenom, sdkmath.OneInt())
	err = pbk.MintCoins(
		suite.network.GetContext(),
		minttypes.ModuleName,
		sdk.NewCoins(coin),
	)
	suite.Require().NoError(err)

	res, err = suite.network.GetPreciseBankClient().Remainder(
		context.Background(),
		&types.QueryRemainderRequest{},
	)
	suite.Require().NoError(err)

	expRemainder.Amount = types.ConversionFactor().Sub(coin.Amount)
	suite.Require().Equal(expRemainder, res.Remainder)
}

func (suite *KeeperIntegrationTestSuite) TestQueryFractionalBalance() {
	testCases := []struct {
		name        string
		giveBalance sdkmath.Int
	}{
		{
			"zero",
			sdkmath.ZeroInt(),
		},
		{
			"min amount",
			sdkmath.OneInt(),
		},
		{
			"max amount",
			types.ConversionFactor().SubRaw(1),
		},
		{
			"multiple integer amounts, 0 fractional",
			types.ConversionFactor().MulRaw(5),
		},
		{
			"multiple integer amounts, non-zero fractional",
			types.ConversionFactor().MulRaw(5).Add(types.ConversionFactor().QuoRaw(2)),
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			addr := sdk.AccAddress([]byte("test"))

			coin := sdk.NewCoin(types.ExtendedCoinDenom, tc.giveBalance)
			suite.MintToAccount(addr, sdk.NewCoins(coin))

			res, err := suite.network.GetPreciseBankClient().FractionalBalance(
				context.Background(),
				&types.QueryFractionalBalanceRequest{
					Address: addr.String(),
				},
			)
			suite.Require().NoError(err)

			// Only fractional amount, even if minted more than conversion factor
			expAmount := tc.giveBalance.Mod(types.ConversionFactor())
			expFractionalBalance := sdk.NewCoin(types.ExtendedCoinDenom, expAmount)
			suite.Require().Equal(expFractionalBalance, res.FractionalBalance)
		})
	}
}
