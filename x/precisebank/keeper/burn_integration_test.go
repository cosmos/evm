package keeper_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/precisebank/keeper"
	"github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func (suite *KeeperIntegrationTestSuite) TestBurnCoins_MatchingErrors() {
	// x/precisebank BurnCoins should be identical to x/bank BurnCoins to
	// consumers. This test ensures that the panics & errors returned by
	// x/precisebank are identical to x/bank.

	tests := []struct {
		name            string
		recipientModule string
		setupFn         func()
		burnAmount      sdk.Coins
		wantErr         string
		wantPanic       string
	}{
		{
			"invalid module",
			"notamodule",
			func() {},
			cs(c("uatom", 1000)),
			"",
			"module account notamodule does not exist: unknown address",
		},
		{
			"no burn permissions",
			// Check app.go to ensure this module has no burn permissions
			authtypes.FeeCollectorName,
			func() {},
			cs(c("uatom", 1000)),
			"",
			"module account fee_collector does not have permissions to burn tokens: unauthorized",
		},
		{
			"invalid amount",
			// Has burn permissions so it goes to the amt check
			ibctransfertypes.ModuleName,
			func() {},
			sdk.Coins{sdk.Coin{Denom: "uatom", Amount: sdkmath.NewInt(-100)}},
			"-100uatom: invalid coins",
			"",
		},
		{
			"insufficient balance - empty",
			ibctransfertypes.ModuleName,
			func() {},
			cs(c("uatom", 1000)),
			"spendable balance 0uatom is smaller than 1000uatom: insufficient funds",
			"",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset
			suite.SetupTest()

			if tt.wantErr == "" && tt.wantPanic == "" {
				suite.Fail("test must specify either wantErr or wantPanic")
			}

			if tt.wantErr != "" {
				// Check x/bank BurnCoins for identical error
				bankErr := suite.network.App.BankKeeper.BurnCoins(suite.network.GetContext(), tt.recipientModule, tt.burnAmount)
				suite.Require().Error(bankErr)
				suite.Require().EqualError(bankErr, tt.wantErr, "expected error should match x/bank BurnCoins error")

				pbankErr := suite.network.App.PreciseBankKeeper.BurnCoins(suite.network.GetContext(), tt.recipientModule, tt.burnAmount)
				suite.Require().Error(pbankErr)
				// Compare strings instead of errors, as error stack is still different
				suite.Require().Equal(
					bankErr.Error(),
					pbankErr.Error(),
					"x/precisebank error should match x/bank BurnCoins error",
				)
			}

			if tt.wantPanic != "" {
				// First check the wantPanic string is correct.
				// Actually specify the panic string in the test since it makes
				// it more clear we are testing specific and different cases.
				suite.Require().PanicsWithError(tt.wantPanic, func() {
					_ = suite.network.App.BankKeeper.BurnCoins(suite.network.GetContext(), tt.recipientModule, tt.burnAmount)
				}, "expected panic error should match x/bank BurnCoins")

				suite.Require().PanicsWithError(tt.wantPanic, func() {
					_ = suite.network.App.PreciseBankKeeper.BurnCoins(suite.network.GetContext(), tt.recipientModule, tt.burnAmount)
				}, "x/precisebank panic should match x/bank BurnCoins")
			}
		})
	}
}

func (suite *KeeperIntegrationTestSuite) TestBurnCoins() {
	tests := []struct {
		name         string
		startBalance sdk.Coins
		burnCoins    sdk.Coins
		wantBalance  sdk.Coins
		wantErr      string
	}{
		{
			"passthrough - unrelated",
			cs(c("meow", 1000)),
			cs(c("meow", 1000)),
			cs(),
			"",
		},
		{
			"passthrough - integer denom",
			cs(c(types.IntegerCoinDenom, 2000)),
			cs(c(types.IntegerCoinDenom, 1000)),
			cs(c(types.ExtendedCoinDenom, 1000000000000000)),
			"",
		},
		{
			"fractional only - no borrow",
			cs(c(types.ExtendedCoinDenom, 1000)),
			cs(c(types.ExtendedCoinDenom, 500)),
			cs(c(types.ExtendedCoinDenom, 500)),
			"",
		},
		{
			"fractional burn - borrows",
			cs(ci(types.ExtendedCoinDenom, types.ConversionFactor().AddRaw(100))),
			cs(c(types.ExtendedCoinDenom, 500)),
			cs(ci(types.ExtendedCoinDenom, types.ConversionFactor().SubRaw(400))),
			"",
		},
		{
			"error - insufficient integer balance",
			cs(ci(types.ExtendedCoinDenom, types.ConversionFactor())),
			cs(ci(types.ExtendedCoinDenom, types.ConversionFactor().MulRaw(2))),
			cs(),
			// Returns correct error with aatom balance (rewrites Bank BurnCoins err)
			"spendable balance 1000000000000aatom is smaller than 2000000000000aatom: insufficient funds",
		},
		{
			"error - insufficient fractional, borrow",
			cs(c(types.ExtendedCoinDenom, 1000)),
			cs(c(types.ExtendedCoinDenom, 2000)),
			cs(),
			// Error from SendCoins to reserve
			"spendable balance 1000aatom is smaller than 2000aatom: insufficient funds",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset
			suite.SetupTest()

			moduleName := ibctransfertypes.ModuleName
			recipientAddr := suite.network.App.AccountKeeper.GetModuleAddress(moduleName)

			// Start balance
			err := suite.network.App.PreciseBankKeeper.MintCoins(suite.network.GetContext(), moduleName, tt.startBalance)
			suite.Require().NoError(err)

			// Burn
			err = suite.network.App.PreciseBankKeeper.BurnCoins(suite.network.GetContext(), moduleName, tt.burnCoins)
			if tt.wantErr != "" {
				suite.Require().Error(err)
				suite.Require().EqualError(err, tt.wantErr)
				return
			}

			suite.Require().NoError(err)

			// -------------------------------------------------------------
			// Check FULL balances
			// x/bank balances + x/precisebank balance
			// Exclude "uatom" as x/precisebank balance will include it
			afterBalance := suite.GetAllBalances(recipientAddr)

			suite.Require().Equal(
				tt.wantBalance.String(),
				afterBalance.String(),
				"unexpected balance after minting %s to %s",
			)

			// Ensure reserve is backing all minted fractions
			allInvariantsFn := keeper.AllInvariants(suite.network.App.PreciseBankKeeper)
			res, stop := allInvariantsFn(suite.network.GetContext())
			suite.Require().False(stop, "invariant should not be broken")
			suite.Require().Empty(res, "unexpected invariant message: %s", res)

			intCoinAmt := tt.burnCoins.AmountOf(types.IntegerCoinDenom).
				Mul(types.ConversionFactor())

			fraCoinAmt := tt.burnCoins.AmountOf(types.ExtendedCoinDenom)

			totalExtCoinAmt := intCoinAmt.Add(fraCoinAmt)
			spentCoins := sdk.NewCoins(sdk.NewCoin(
				types.ExtendedCoinDenom,
				totalExtCoinAmt,
			))

			events := suite.network.GetContext().EventManager().Events()

			expBurnEvent := banktypes.NewCoinBurnEvent(recipientAddr, spentCoins)
			expSpendEvent := banktypes.NewCoinSpentEvent(recipientAddr, spentCoins)

			if totalExtCoinAmt.IsZero() {
				suite.Require().NotContains(events, expBurnEvent)
				suite.Require().NotContains(events, expSpendEvent)
			} else {
				suite.Require().Contains(events, expBurnEvent)
				suite.Require().Contains(events, expSpendEvent)
			}
		})
	}
}

func (suite *KeeperIntegrationTestSuite) TestBurnCoins_Remainder() {
	// This tests a series of small burns to ensure the remainder is both
	// updated correctly and reserve is correctly updated. This only burns from
	// 1 single account.

	reserveAddr := suite.network.App.AccountKeeper.GetModuleAddress(types.ModuleName)

	moduleName := ibctransfertypes.ModuleName
	moduleAddr := suite.network.App.AccountKeeper.GetModuleAddress(moduleName)

	startCoins := cs(ci(types.ExtendedCoinDenom, types.ConversionFactor().MulRaw(5)))

	// Start balance
	err := suite.network.App.PreciseBankKeeper.MintCoins(
		suite.network.GetContext(),
		moduleName,
		startCoins,
	)
	suite.Require().NoError(err)

	burnAmt := types.ConversionFactor().QuoRaw(10)
	burnCoins := cs(ci(types.ExtendedCoinDenom, burnAmt))

	// Burn 0.1 until balance is 0
	for {
		reserveBalBefore := suite.network.App.BankKeeper.GetBalance(
			suite.network.GetContext(),
			reserveAddr,
			types.IntegerCoinDenom,
		)

		balBefore := suite.network.App.PreciseBankKeeper.GetBalance(
			suite.network.GetContext(),
			moduleAddr,
			types.ExtendedCoinDenom,
		)
		remainderBefore := suite.network.App.PreciseBankKeeper.GetRemainderAmount(suite.network.GetContext())

		// ----------------------------------------
		// Burn
		err := suite.network.App.PreciseBankKeeper.BurnCoins(
			suite.network.GetContext(),
			moduleName,
			burnCoins,
		)
		suite.Require().NoError(err)

		// ----------------------------------------
		// Checks
		remainderAfter := suite.network.App.PreciseBankKeeper.GetRemainderAmount(suite.network.GetContext())
		balAfter := suite.network.App.PreciseBankKeeper.GetBalance(
			suite.network.GetContext(),
			moduleAddr,
			types.ExtendedCoinDenom,
		)
		reserveBalAfter := suite.network.App.BankKeeper.GetBalance(
			suite.network.GetContext(),
			reserveAddr,
			types.IntegerCoinDenom,
		)

		suite.Require().Equal(
			balBefore.Amount.Sub(burnAmt).String(),
			balAfter.Amount.String(),
			"balance should decrease by burn amount",
		)

		// Remainder should be updated correctly
		suite.Require().Equal(
			remainderBefore.Add(burnAmt).Mod(types.ConversionFactor()),
			remainderAfter,
		)

		// If remainder has exceeded (then rolled over), reserve should be updated
		if remainderAfter.LT(remainderBefore) {
			suite.Require().Equal(
				reserveBalBefore.Amount.SubRaw(1).String(),
				reserveBalAfter.Amount.String(),
				"reserve should decrease by 1 if remainder exceeds ConversionFactor",
			)
		}

		// No more to burn
		if balAfter.Amount.IsZero() {
			break
		}
	}

	// Run Invariants to ensure remainder is backing all fractions correctly
	res, stop := keeper.AllInvariants(suite.network.App.PreciseBankKeeper)(suite.network.GetContext())
	suite.Require().False(stop, "invariant should not be broken")
	suite.Require().Empty(res, "unexpected invariant message: %s", res)
}

func (suite *KeeperIntegrationTestSuite) TestBurnCoins_Spread_Remainder() {
	// This tests a series of small burns to ensure the remainder is both
	// updated correctly and reserve is correctly updated. This burns from
	// a series of multiple accounts, to test when the remainder is modified
	// by multiple accounts.

	reserveAddr := suite.network.App.AccountKeeper.GetModuleAddress(types.ModuleName)
	burnerModuleName := ibctransfertypes.ModuleName
	burnerAddr := suite.network.App.AccountKeeper.GetModuleAddress(burnerModuleName)

	accCount := 20
	startCoins := cs(ci(types.ExtendedCoinDenom, types.ConversionFactor().MulRaw(5)))

	addrs := []sdk.AccAddress{}

	for i := 0; i < accCount; i++ {
		addr := sdk.AccAddress(fmt.Sprintf("addr%d", i))
		suite.MintToAccount(addr, startCoins)

		addrs = append(addrs, addr)
	}

	burnAmt := types.ConversionFactor().QuoRaw(10)
	burnCoins := cs(ci(types.ExtendedCoinDenom, burnAmt))

	// Burn 0.1 from each account
	for _, addr := range addrs {
		reserveBalBefore := suite.network.App.BankKeeper.GetBalance(
			suite.network.GetContext(),
			reserveAddr,
			types.IntegerCoinDenom,
		)

		balBefore := suite.network.App.PreciseBankKeeper.GetBalance(
			suite.network.GetContext(),
			addr,
			types.ExtendedCoinDenom,
		)
		remainderBefore := suite.network.App.PreciseBankKeeper.GetRemainderAmount(suite.network.GetContext())

		// ----------------------------------------
		// Send & Burn
		err := suite.network.App.PreciseBankKeeper.SendCoins(
			suite.network.GetContext(),
			addr,
			burnerAddr,
			burnCoins,
		)
		suite.Require().NoError(err)

		err = suite.network.App.PreciseBankKeeper.BurnCoins(
			suite.network.GetContext(),
			burnerModuleName,
			burnCoins,
		)
		suite.Require().NoError(err)

		// ----------------------------------------
		// Checks
		remainderAfter := suite.network.App.PreciseBankKeeper.GetRemainderAmount(suite.network.GetContext())
		balAfter := suite.network.App.PreciseBankKeeper.GetBalance(
			suite.network.GetContext(),
			addr,
			types.ExtendedCoinDenom,
		)
		reserveBalAfter := suite.network.App.BankKeeper.GetBalance(
			suite.network.GetContext(),
			reserveAddr,
			types.IntegerCoinDenom,
		)

		suite.Require().Equal(
			balBefore.Amount.Sub(burnAmt).String(),
			balAfter.Amount.String(),
			"balance should decrease by burn amount",
		)

		// Remainder should be updated correctly
		suite.Require().Equal(
			remainderBefore.Add(burnAmt).Mod(types.ConversionFactor()),
			remainderAfter,
		)

		suite.T().Logf("acc: %s", string(addr.Bytes()))
		suite.T().Logf("acc bal: %s -> %s", balBefore, balAfter)
		suite.T().Logf("remainder: %s -> %s", remainderBefore, remainderAfter)
		suite.T().Logf("reserve: %v -> %v", reserveBalBefore, reserveBalAfter)

		// Reserve will change when:
		// 1. Account needs to borrow from integer (transfers to reserve)
		// 2. Remainder meets or exceeds conversion factor (burn 1 from reserve)
		reserveIncrease := sdkmath.ZeroInt()

		// Does account need to borrow from integer?
		if balBefore.Amount.Mod(types.ConversionFactor()).LT(burnAmt) {
			reserveIncrease = reserveIncrease.AddRaw(1)
		}

		// If remainder has exceeded (then rolled over), burn additional 1
		if remainderBefore.Add(burnAmt).GTE(types.ConversionFactor()) {
			reserveIncrease = reserveIncrease.SubRaw(1)
		}

		suite.Require().Equal(
			reserveBalBefore.Amount.Add(reserveIncrease).String(),
			reserveBalAfter.Amount.String(),
			"reserve should be updated by remainder and borrowing",
		)

		// Run Invariants to ensure remainder is backing all fractions correctly
		res, stop := keeper.AllInvariants(suite.network.App.PreciseBankKeeper)(suite.network.GetContext())
		suite.Require().False(stop, "invariant should not be broken")
		suite.Require().Empty(res, "unexpected invariant message: %s", res)
	}
}

func FuzzBurnCoins(f *testing.F) {
	evmConfigurator := evmtypes.NewEVMConfigurator()
	evmConfigurator.WithEVMCoinInfo(testconstants.ExampleMicroDenom, uint8(evmtypes.SixDecimals))
	err := evmConfigurator.Configure()
	require.NoError(f, err)

	f.Add(int64(0))
	f.Add(int64(100))
	f.Add(types.ConversionFactor().Int64())
	f.Add(types.ConversionFactor().MulRaw(5).Int64())
	f.Add(types.ConversionFactor().MulRaw(2).AddRaw(123948723).Int64())

	f.Fuzz(func(t *testing.T, amount int64) {
		// No negative amounts
		if amount < 0 {
			amount = -amount
		}

		// Manually setup test suite since no direct Fuzz support in test suites
		suite := new(KeeperIntegrationTestSuite)
		suite.SetT(t)
		suite.SetS(suite)
		suite.SetupTest()

		burnCount := int64(10)

		// Has both mint & burn permissions
		moduleName := ibctransfertypes.ModuleName
		recipientAddr := suite.network.App.AccountKeeper.GetModuleAddress(moduleName)

		// Start balance
		err := suite.network.App.PreciseBankKeeper.MintCoins(
			suite.network.GetContext(),
			moduleName,
			cs(ci(types.ExtendedCoinDenom, sdkmath.NewInt(amount).MulRaw(burnCount))),
		)
		suite.Require().NoError(err)

		// Burn multiple times to ensure different balance scenarios
		for i := int64(0); i < burnCount; i++ {
			err := suite.network.App.PreciseBankKeeper.BurnCoins(
				suite.network.GetContext(),
				moduleName,
				cs(c(types.ExtendedCoinDenom, amount)),
			)
			suite.Require().NoError(err)
		}

		// Check FULL balances
		balAfter := suite.network.App.PreciseBankKeeper.GetBalance(suite.network.GetContext(), recipientAddr, types.ExtendedCoinDenom)

		suite.Require().Equalf(
			int64(0),
			balAfter.Amount.Int64(),
			"all coins should be burned, got %d",
			balAfter.Amount.Int64(),
		)

		// Run Invariants to ensure remainder is backing all fractions correctly
		allInvariantsFn := keeper.AllInvariants(suite.network.App.PreciseBankKeeper)
		res, stop := allInvariantsFn(suite.network.GetContext())
		suite.Require().False(stop, "invariant should not be broken")
		suite.Require().Empty(res, "unexpected invariant message: %s", res)
	})
}
