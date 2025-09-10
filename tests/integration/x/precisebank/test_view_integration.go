package precisebank

import (
	testconfig "github.com/cosmos/evm/testutil/config"
	"github.com/cosmos/evm/x/precisebank/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
)

func (s *KeeperIntegrationTestSuite) TestKeeperSpendableCoin() {
	integerDenom := s.network.GetEVMDenom()
	extendedDenom := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.GetExtendedDenom()
	extendedDecimals := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.ExtendedDecimals

	tests := []struct {
		name      string
		giveDenom string // queried denom for balance

		giveBankBal       sdk.Coins   // full balance
		giveFractionalBal sdkmath.Int // stored fractional balance for giveAddr
		giveLockedCoins   sdk.Coins   // locked coins

		wantSpendableBal sdk.Coin
	}{
		{
			"extended denom, no fractional - locked coins",
			extendedDenom,
			// queried bank balance in uatom when querying for aatom
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(1000))),
			sdkmath.ZeroInt(),
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(10))),
			// (integer + fractional) - locked
			sdk.NewCoin(
				extendedDenom,
				types.ConversionFactor(extendedDecimals).MulRaw(1000-10),
			),
		},
		{
			"extended denom, with fractional - locked coins",
			extendedDenom,
			// queried bank balance in uatom when querying for aatom
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(1000))),
			sdkmath.NewInt(5000),
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(10))),
			sdk.NewCoin(
				extendedDenom,
				// (integer - locked) + fractional
				types.ConversionFactor(extendedDecimals).MulRaw(1000-10).AddRaw(5000),
			),
		},
		{
			"non-extended denom - uatom returns uatom",
			integerDenom,
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(1000))),
			sdkmath.ZeroInt(),
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(10))),
			sdk.NewCoin(integerDenom, sdkmath.NewInt(990)),
		},
		{
			"non-extended denom, with fractional - uatom returns uatom",
			integerDenom,
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(1000))),
			// does not affect balance
			sdkmath.NewInt(100),
			sdk.NewCoins(sdk.NewCoin(integerDenom, sdkmath.NewInt(10))),
			sdk.NewCoin(integerDenom, sdkmath.NewInt(990)),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			addr := sdk.AccAddress([]byte("test-address"))

			s.MintToAccount(addr, tt.giveBankBal)

			// Set fractional balance in store before query
			s.network.App.GetPreciseBankKeeper().SetFractionalBalance(s.network.GetContext(), addr, tt.giveFractionalBal)

			// Add some locked coins
			acc := s.network.App.GetAccountKeeper().GetAccount(s.network.GetContext(), addr)
			if acc == nil {
				acc = authtypes.NewBaseAccount(addr, nil, 0, 0)
			}

			vestingAcc, err := vestingtypes.NewPeriodicVestingAccount(
				acc.(*authtypes.BaseAccount),
				tt.giveLockedCoins,
				s.network.GetContext().BlockTime().Unix(),
				vestingtypes.Periods{
					vestingtypes.Period{
						Length: 100,
						Amount: tt.giveLockedCoins,
					},
				},
			)
			s.Require().NoError(err)
			s.network.App.GetAccountKeeper().SetAccount(s.network.GetContext(), vestingAcc)

			fetchedLockedCoins := vestingAcc.LockedCoins(s.network.GetContext().BlockTime())
			s.Require().Equal(
				tt.giveLockedCoins,
				fetchedLockedCoins,
				"locked coins should be matching at current block time",
			)

			spendableCoinsWithLocked := s.network.App.GetPreciseBankKeeper().SpendableCoin(s.network.GetContext(), addr, tt.giveDenom)

			s.Require().Equalf(
				tt.wantSpendableBal,
				spendableCoinsWithLocked,
				"expected spendable coins of denom %s",
				tt.giveDenom,
			)
		})
	}
}

func (s *KeeperIntegrationTestSuite) TestKeeperHiddenReserve() {
	// Reserve balances should not be shown to consumers of x/precisebank, as it
	// represents the fractional balances of accounts.

	integerDenom := s.network.GetEVMDenom()
	extendedDenom := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.GetExtendedDenom()
	extendedDecimals := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.ExtendedDecimals

	moduleAddr := authtypes.NewModuleAddress(types.ModuleName)
	addr1 := sdk.AccAddress{1}

	// Make the reserve hold a non-zero balance
	// Mint fractional coins to an account, which should cause a mint of 1
	// integer coin to the reserve to back it.
	extCoin := sdk.NewCoin(extendedDenom, types.ConversionFactor(extendedDecimals).AddRaw(1000))
	unrelatedCoin := sdk.NewCoin("unrelated", sdkmath.NewInt(1000))
	s.MintToAccount(
		addr1,
		sdk.NewCoins(
			extCoin,
			unrelatedCoin,
		),
	)

	// Check underlying x/bank balance for reserve
	reserveIntCoin := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), moduleAddr, integerDenom)
	s.Require().Equal(
		sdkmath.NewInt(2), // Network setup creates 1, test mints 1 more = 2 total
		reserveIntCoin.Amount,
		"reserve should hold 2 integer coins (1 from network setup + 1 from test mint)",
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
			extendedDenom,
			sdkmath.ZeroInt(),
		},
		{
			"reserve account - visible integer denom",
			moduleAddr,
			integerDenom,
			sdkmath.NewInt(2), // Network setup creates 1, test mints 1 more = 2 total
		},
		{
			"user account - visible extended denom",
			addr1,
			extendedDenom,
			extCoin.Amount,
		},
		{
			"user account - visible integer denom",
			addr1,
			integerDenom,
			extCoin.Amount.Quo(types.ConversionFactor(extendedDecimals)),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			coin := s.network.App.GetPreciseBankKeeper().GetBalance(s.network.GetContext(), tt.giveAddr, tt.giveDenom)
			s.Require().Equal(tt.wantAmount.Int64(), coin.Amount.Int64())

			spendableCoin := s.network.App.GetPreciseBankKeeper().SpendableCoin(s.network.GetContext(), tt.giveAddr, tt.giveDenom)
			s.Require().Equal(tt.wantAmount.Int64(), spendableCoin.Amount.Int64())
		})
	}
}
