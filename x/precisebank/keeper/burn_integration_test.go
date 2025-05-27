package keeper_test

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

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
			cs(c(types.IntegerCoinDenom(), 1000)),
			"",
			"module account notamodule does not exist: unknown address",
		},
		{
			"no burn permissions",
			// Check app.go to ensure this module has no burn permissions
			authtypes.FeeCollectorName,
			func() {},
			cs(c(types.IntegerCoinDenom(), 1000)),
			"",
			"module account fee_collector does not have permissions to burn tokens: unauthorized",
		},
		{
			"invalid amount",
			// Has burn permissions so it goes to the amt check
			evmtypes.ModuleName,
			func() {},
			sdk.Coins{sdk.Coin{Denom: types.IntegerCoinDenom(), Amount: sdkmath.NewInt(-100)}},
			fmt.Sprintf("-100%s: invalid coins", types.IntegerCoinDenom()),
			"",
		},
		{
			"insufficient balance - empty",
			evmtypes.ModuleName,
			func() {},
			cs(c(types.IntegerCoinDenom(), 1000)),
			fmt.Sprintf("spendable balance 0%s is smaller than 1000%s: insufficient funds", types.IntegerCoinDenom(), types.IntegerCoinDenom()),
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
			cs(c(types.IntegerCoinDenom(), 2000)),
			cs(c(types.IntegerCoinDenom(), 1000)),
			cs(c(types.ExtendedCoinDenom(), 1000000000000000)),
			"",
		},
		{
			"fractional only - no borrow",
			cs(c(types.ExtendedCoinDenom(), 1000)),
			cs(c(types.ExtendedCoinDenom(), 500)),
			cs(c(types.ExtendedCoinDenom(), 500)),
			"",
		},
		{
			"fractional burn - borrows",
			cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor().AddRaw(100))),
			cs(c(types.ExtendedCoinDenom(), 500)),
			cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor().SubRaw(400))),
			"",
		},
		{
			"error - insufficient integer balance",
			cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor())),
			cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor().MulRaw(2))),
			cs(),
			// Returns correct error with aatom balance (rewrites Bank BurnCoins err)
			fmt.Sprintf("spendable balance 1000000000000%s is smaller than 2000000000000%s: insufficient funds",
				types.ExtendedCoinDenom(), types.ExtendedCoinDenom()),
		},
		{
			"error - insufficient fractional, borrow",
			cs(c(types.ExtendedCoinDenom(), 1000)),
			cs(c(types.ExtendedCoinDenom(), 2000)),
			cs(),
			// Error from SendCoins to reserve
			fmt.Sprintf("spendable balance 1000%s is smaller than 2000%s: insufficient funds",
				types.ExtendedCoinDenom(), types.ExtendedCoinDenom()),
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset
			suite.SetupTest()

			moduleName := evmtypes.ModuleName
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

			intCoinAmt := tt.burnCoins.AmountOf(types.IntegerCoinDenom()).
				Mul(types.ConversionFactor())

			fraCoinAmt := tt.burnCoins.AmountOf(types.ExtendedCoinDenom())

			totalExtCoinAmt := intCoinAmt.Add(fraCoinAmt)
			spentCoins := sdk.NewCoins(sdk.NewCoin(
				types.ExtendedCoinDenom(),
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

	moduleName := evmtypes.ModuleName
	moduleAddr := suite.network.App.AccountKeeper.GetModuleAddress(moduleName)

	startCoins := cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor().MulRaw(5)))

	// Start balance
	err := suite.network.App.PreciseBankKeeper.MintCoins(
		suite.network.GetContext(),
		moduleName,
		startCoins,
	)
	suite.Require().NoError(err)

	burnAmt := types.ConversionFactor().QuoRaw(10)
	burnCoins := cs(ci(types.ExtendedCoinDenom(), burnAmt))

	// Burn 0.1 until balance is 0
	for {
		reserveBalBefore := suite.network.App.BankKeeper.GetBalance(
			suite.network.GetContext(),
			reserveAddr,
			types.IntegerCoinDenom(),
		)

		balBefore := suite.network.App.PreciseBankKeeper.GetBalance(
			suite.network.GetContext(),
			moduleAddr,
			types.ExtendedCoinDenom(),
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
			types.ExtendedCoinDenom(),
		)
		reserveBalAfter := suite.network.App.BankKeeper.GetBalance(
			suite.network.GetContext(),
			reserveAddr,
			types.IntegerCoinDenom(),
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
}

func (suite *KeeperIntegrationTestSuite) TestBurnCoins_Spread_Remainder() {
	// This tests a series of small burns to ensure the remainder is both
	// updated correctly and reserve is correctly updated. This burns from
	// a series of multiple accounts, to test when the remainder is modified
	// by multiple accounts.

	reserveAddr := suite.network.App.AccountKeeper.GetModuleAddress(types.ModuleName)
	burnerModuleName := evmtypes.ModuleName
	burnerAddr := suite.network.App.AccountKeeper.GetModuleAddress(burnerModuleName)

	accCount := 20
	startCoins := cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor().MulRaw(5)))

	addrs := []sdk.AccAddress{}

	for i := 0; i < accCount; i++ {
		addr := sdk.AccAddress(fmt.Sprintf("addr%d", i))
		suite.MintToAccount(addr, startCoins)

		addrs = append(addrs, addr)
	}

	burnAmt := types.ConversionFactor().QuoRaw(10)
	burnCoins := cs(ci(types.ExtendedCoinDenom(), burnAmt))

	// Burn 0.1 from each account
	for _, addr := range addrs {
		reserveBalBefore := suite.network.App.BankKeeper.GetBalance(
			suite.network.GetContext(),
			reserveAddr,
			types.IntegerCoinDenom(),
		)

		balBefore := suite.network.App.PreciseBankKeeper.GetBalance(
			suite.network.GetContext(),
			addr,
			types.ExtendedCoinDenom(),
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
			types.ExtendedCoinDenom(),
		)
		reserveBalAfter := suite.network.App.BankKeeper.GetBalance(
			suite.network.GetContext(),
			reserveAddr,
			types.IntegerCoinDenom(),
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
	}
}

func (suite *KeeperIntegrationTestSuite) TestBurnCoins_RandomValueMultiDecimals() {
	tests := []struct {
		name    string
		chainID testconstants.ChainID
	}{
		{
			name:    "6 decimals",
			chainID: testconstants.SixDecimalsChainID,
		},
		{
			name:    "12 decimals",
			chainID: testconstants.TwelveDecimalsChainID,
		},
		{
			name:    "2 decimals",
			chainID: testconstants.TwoDecimalsChainID,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.SetupTestWithChainID(tt.chainID)

			// Has burn permissions
			burnerModuleName := evmtypes.ModuleName
			burner := sdk.AccAddress([]byte{1})

			// Initial balance large enough to cover many small burns
			initialBalance := types.ConversionFactor().MulRaw(100)
			initialCoin := cs(ci(types.ExtendedCoinDenom(), initialBalance))
			err := suite.network.App.PreciseBankKeeper.MintCoins(suite.network.GetContext(), burnerModuleName, initialCoin)
			suite.Require().NoError(err)
			err = suite.network.App.PreciseBankKeeper.SendCoinsFromModuleToAccount(suite.network.GetContext(), burnerModuleName, burner, initialCoin)
			suite.Require().NoError(err)

			// Setup test parameters
			maxBurnUnit := types.ConversionFactor().MulRaw(2).SubRaw(1)
			r := rand.New(rand.NewSource(SEED))

			totalBurned := sdkmath.ZeroInt()
			burnCount := 0

			// Continue burns as long as burner has balance remaining
			for {
				// Check current burner balance
				burnerAmount := suite.GetAllBalances(burner).AmountOf(types.ExtendedCoinDenom())
				if burnerAmount.IsZero() {
					break
				}

				// Generate random amount within the range of max possible burn amount
				maxPossibleBurn := maxBurnUnit
				if maxPossibleBurn.GT(burnerAmount) {
					maxPossibleBurn = burnerAmount
				}
				randAmount := sdkmath.NewIntFromBigInt(new(big.Int).Rand(r, maxPossibleBurn.BigInt())).AddRaw(1)

				// 1. send to burner module
				burnCoins := cs(ci(types.ExtendedCoinDenom(), randAmount))
				err := suite.network.App.PreciseBankKeeper.SendCoinsFromAccountToModule(suite.network.GetContext(), burner, burnerModuleName, burnCoins)
				suite.Require().NoError(err)

				// 2. burn from burner module
				err = suite.network.App.PreciseBankKeeper.BurnCoins(suite.network.GetContext(), burnerModuleName, burnCoins)
				suite.Require().NoError(err)

				totalBurned = totalBurned.Add(randAmount)
				burnCount++
			}

			suite.T().Logf("Completed %d random burns, total burned: %s", burnCount, totalBurned)

			// Check burner balance
			burnerBal := suite.GetAllBalances(burner).AmountOf(types.ExtendedCoinDenom())
			suite.Equal(burnerBal.BigInt().Cmp(big.NewInt(0)), 0, "burner balance mismatch (expected: %s, actual: %s)", big.NewInt(0), burnerBal)

			// Check remainder
			remainder := suite.network.App.PreciseBankKeeper.GetRemainderAmount(suite.network.GetContext())
			suite.Equal(remainder.BigInt().Cmp(big.NewInt(0)), 0, "remainder should be zero (expected: %s, actual: %s)", big.NewInt(0), remainder)
		})
	}
}

func FuzzBurnCoins(f *testing.F) {
	configurator := evmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	configurator.WithEVMCoinInfo(testconstants.ExampleChainCoinInfo[testconstants.SixDecimalsChainID])
	err := configurator.Configure()
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
		moduleName := evmtypes.ModuleName
		moduleAddr := suite.network.App.AccountKeeper.GetModuleAddress(moduleName)

		// Start balance
		err := suite.network.App.PreciseBankKeeper.MintCoins(
			suite.network.GetContext(),
			moduleName,
			cs(ci(types.ExtendedCoinDenom(), sdkmath.NewInt(amount).MulRaw(burnCount))),
		)
		suite.Require().NoError(err)

		// Burn multiple times to ensure different balance scenarios
		for i := int64(0); i < burnCount; i++ {
			err := suite.network.App.PreciseBankKeeper.BurnCoins(
				suite.network.GetContext(),
				moduleName,
				cs(c(types.ExtendedCoinDenom(), amount)),
			)
			suite.Require().NoError(err)
		}

		// Check full balances
		balAfter := suite.network.App.PreciseBankKeeper.GetBalance(suite.network.GetContext(), moduleAddr, types.ExtendedCoinDenom())

		suite.Require().Equalf(
			int64(0),
			balAfter.Amount.Int64(),
			"all coins should be burned, got %d",
			balAfter.Amount.Int64(),
		)
	})
}
