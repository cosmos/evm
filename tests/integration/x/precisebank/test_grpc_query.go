package precisebank

import (
	"context"

	"github.com/cosmos/evm/x/precisebank/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

func (s *KeeperIntegrationTestSuite) TestQueryRemainder() {
	res, err := s.network.GetPreciseBankClient().Remainder(
		context.Background(),
		&types.QueryRemainderRequest{},
	)
	s.Require().NoError(err)

	expRemainder := sdk.NewCoin(types.ExtendedCoinDenom(), sdkmath.ZeroInt())
	s.Require().Equal(expRemainder, res.Remainder)

	// Mint fractional coins to create non-zero remainder

	pbk := s.network.App.GetPreciseBankKeeper()

	coin := sdk.NewCoin(types.ExtendedCoinDenom(), sdkmath.OneInt())
	err = pbk.MintCoins(
		s.network.GetContext(),
		minttypes.ModuleName,
		sdk.NewCoins(coin),
	)
	s.Require().NoError(err)

	res, err = s.network.GetPreciseBankClient().Remainder(
		context.Background(),
		&types.QueryRemainderRequest{},
	)
	s.Require().NoError(err)

	expRemainder.Amount = types.ConversionFactor().Sub(coin.Amount)
	s.Require().Equal(expRemainder, res.Remainder)
}

func (s *KeeperIntegrationTestSuite) TestQueryFractionalBalance() {
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
		s.Run(tc.name, func() {
			s.SetupTest()

			addr := sdk.AccAddress([]byte("test"))

			coin := sdk.NewCoin(types.ExtendedCoinDenom(), tc.giveBalance)
			s.MintToAccount(addr, sdk.NewCoins(coin))

			res, err := s.network.GetPreciseBankClient().FractionalBalance(
				context.Background(),
				&types.QueryFractionalBalanceRequest{
					Address: addr.String(),
				},
			)
			s.Require().NoError(err)

			// Only fractional amount, even if minted more than conversion factor
			expAmount := tc.giveBalance.Mod(types.ConversionFactor())
			expFractionalBalance := sdk.NewCoin(types.ExtendedCoinDenom(), expAmount)
			s.Require().Equal(expFractionalBalance, res.FractionalBalance)
		})
	}
}
