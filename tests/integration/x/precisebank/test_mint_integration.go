package precisebank

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	testconfig "github.com/cosmos/evm/testutil/config"
	"github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func (s *KeeperIntegrationTestSuite) TestBlockedRecipient() {
	// Tests that sending funds to x/precisebank is disallowed.
	// x/precisebank balance is used as the reserve funds and should not be
	// directly interacted with by external modules or users.
	msgServer := bankkeeper.NewMsgServerImpl(s.network.App.GetBankKeeper())

	fromAddr := sdk.AccAddress{1}

	// To x/precisebank
	toAddr := s.network.App.GetAccountKeeper().GetModuleAddress(types.ModuleName)
	integerDenom := s.network.GetEVMDenom()
	amount := cs(c(integerDenom, 1000))

	msg := banktypes.NewMsgSend(fromAddr, toAddr, amount)

	_, err := msgServer.Send(s.network.GetContext(), msg)
	s.Require().Error(err)

	s.Require().EqualError(
		err,
		fmt.Sprintf("%s is not allowed to receive funds: unauthorized", toAddr.String()),
	)
}

func (s *KeeperIntegrationTestSuite) TestMintCoinsMatchingErrors() {
	// x/precisebank MintCoins should be identical to x/bank MintCoins to
	// consumers. This test ensures that the panics & errors returned by
	// x/precisebank are identical to x/bank.

	integerDenom := s.network.GetEVMDenom()

	tests := []struct {
		name            string
		recipientModule string
		mintAmount      sdk.Coins
		wantErr         string
		wantPanic       string
	}{
		{
			"invalid module",
			"notamodule",
			cs(c(integerDenom, 1000)),
			"",
			"module account notamodule does not exist: unknown address",
		},
		{
			"no mint permissions",
			// Check app.go to ensure this module has no mint permissions
			authtypes.FeeCollectorName,
			cs(c(integerDenom, 1000)),
			"",
			"module account fee_collector does not have permissions to mint tokens: unauthorized",
		},
		{
			"invalid amount",
			evmtypes.ModuleName,
			sdk.Coins{sdk.Coin{Denom: integerDenom, Amount: sdkmath.NewInt(-100)}},
			fmt.Sprintf("-100%s: invalid coins", integerDenom),
			"",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// Reset
			s.SetupTest()

			if tt.wantErr == "" && tt.wantPanic == "" {
				s.Fail("test must specify either wantErr or wantPanic")
			}

			if tt.wantErr != "" {
				// Check x/bank MintCoins for identical error
				bankErr := s.network.App.GetBankKeeper().MintCoins(s.network.GetContext(), tt.recipientModule, tt.mintAmount)
				s.Require().Error(bankErr)
				s.Require().EqualError(bankErr, tt.wantErr, "expected error should match x/bank MintCoins error")

				pbankErr := s.network.App.GetPreciseBankKeeper().MintCoins(s.network.GetContext(), tt.recipientModule, tt.mintAmount)
				s.Require().Error(pbankErr)
				// Compare strings instead of errors, as error stack is still different
				s.Require().Equal(
					bankErr.Error(),
					pbankErr.Error(),
					"x/precisebank error should match x/bank MintCoins error",
				)
			}

			if tt.wantPanic != "" {
				// First check the wantPanic string is correct.
				// Actually specify the panic string in the test since it makes
				// it more clear we are testing specific and different cases.
				s.Require().PanicsWithError(tt.wantPanic, func() {
					_ = s.network.App.GetBankKeeper().MintCoins(s.network.GetContext(), tt.recipientModule, tt.mintAmount)
				}, "expected panic error should match x/bank MintCoins")

				s.Require().PanicsWithError(tt.wantPanic, func() {
					_ = s.network.App.GetPreciseBankKeeper().MintCoins(s.network.GetContext(), tt.recipientModule, tt.mintAmount)
				}, "x/precisebank panic should match x/bank MintCoins")
			}
		})
	}
}

func (s *KeeperIntegrationTestSuite) TestMintCoins() {
	type mintTest struct {
		mintAmount sdk.Coins
		// Expected **full** balances after MintCoins(mintAmount)
		wantBalance sdk.Coins
	}

	integerDenom := s.network.GetEVMDenom()
	extendedDenom := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.GetExtendedDenom()
	extendedDecimals := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.ExtendedDecimals

	tests := []struct {
		name            string
		recipientModule string
		// Instead of having a start balance, we just have a list of mints to
		// both test & get into desired non-default states.
		mints []mintTest
	}{
		{
			"passthrough - unrelated",
			evmtypes.ModuleName,
			[]mintTest{
				{
					mintAmount:  cs(c("busd", 1000)),
					wantBalance: cs(c("busd", 1000)),
				},
			},
		},
		{
			"passthrough - integer denom",
			evmtypes.ModuleName,
			[]mintTest{
				{
					mintAmount:  cs(c(integerDenom, 1000)),
					wantBalance: cs(c(extendedDenom, 1000000000000000)),
				},
			},
		},
		{
			"fractional only",
			evmtypes.ModuleName,
			[]mintTest{
				{
					mintAmount:  cs(c(extendedDenom, 1000)),
					wantBalance: cs(c(extendedDenom, 1000)),
				},
				{
					mintAmount:  cs(c(extendedDenom, 1000)),
					wantBalance: cs(c(extendedDenom, 2000)),
				},
			},
		},
		{
			"fractional only with carry",
			evmtypes.ModuleName,
			[]mintTest{
				{
					// Start with (1/4 * 3) = 0.75
					mintAmount:  cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).QuoRaw(4).MulRaw(3))),
					wantBalance: cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).QuoRaw(4).MulRaw(3))),
				},
				{
					// Add another 0.50 to incur carry to test reserve on carry
					mintAmount:  cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).QuoRaw(2))),
					wantBalance: cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).QuoRaw(4).MulRaw(5))),
				},
			},
		},
		{
			"fractional only, resulting in exact carry and 0 remainder",
			evmtypes.ModuleName,
			[]mintTest{
				// mint 0.5, acc = 0.5, reserve = 1
				{
					mintAmount:  cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).QuoRaw(2))),
					wantBalance: cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).QuoRaw(2))),
				},
				// mint another 0.5, acc = 1, reserve = 0
				// Reserve actually goes down by 1 for integer carry
				{
					mintAmount:  cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).QuoRaw(2))),
					wantBalance: cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals))),
				},
			},
		},
		{
			"exact carry",
			evmtypes.ModuleName,
			[]mintTest{
				{
					mintAmount:  cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals))),
					wantBalance: cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals))),
				},
				// Carry again - exact amount
				{
					mintAmount:  cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals))),
					wantBalance: cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).MulRaw(2))),
				},
			},
		},
		{
			"carry with extra",
			evmtypes.ModuleName,
			[]mintTest{
				// MintCoins(C + 100)
				{
					mintAmount:  cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).AddRaw(100))),
					wantBalance: cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).AddRaw(100))),
				},
				// MintCoins(C + 5), total = 2C + 105
				{
					mintAmount:  cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).AddRaw(5))),
					wantBalance: cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).MulRaw(2).AddRaw(105))),
				},
			},
		},
		{
			"integer with fractional",
			evmtypes.ModuleName,
			[]mintTest{
				{
					mintAmount:  cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).MulRaw(5).AddRaw(100))),
					wantBalance: cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).MulRaw(5).AddRaw(100))),
				},
				{
					mintAmount:  cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).MulRaw(2).AddRaw(5))),
					wantBalance: cs(ci(extendedDenom, types.ConversionFactor(extendedDecimals).MulRaw(7).AddRaw(105))),
				},
			},
		},
		{
			"with passthrough",
			evmtypes.ModuleName,
			[]mintTest{
				{
					mintAmount: cs(
						ci(extendedDenom, types.ConversionFactor(extendedDecimals).MulRaw(5).AddRaw(100)),
						c("busd", 1000),
					),
					wantBalance: cs(
						ci(extendedDenom, types.ConversionFactor(extendedDecimals).MulRaw(5).AddRaw(100)),
						c("busd", 1000),
					),
				},
				{
					mintAmount: cs(
						ci(extendedDenom, types.ConversionFactor(extendedDecimals).MulRaw(2).AddRaw(5)),
						c("meow", 40),
					),
					wantBalance: cs(
						ci(extendedDenom, types.ConversionFactor(extendedDecimals).MulRaw(7).AddRaw(105)),
						c("busd", 1000),
						c("meow", 40),
					),
				},
			},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// Reset
			s.SetupTest()

			recipientAddr := s.network.App.GetAccountKeeper().GetModuleAddress(tt.recipientModule)

			for _, mt := range tt.mints {
				// Get fractional balance before mint
				fracBalBefore := s.network.App.GetPreciseBankKeeper().GetFractionalBalance(s.network.GetContext(), recipientAddr)

				err := s.network.App.GetPreciseBankKeeper().MintCoins(s.network.GetContext(), tt.recipientModule, mt.mintAmount)
				s.Require().NoError(err)

				// -------------------------------------------------------------
				// Check FULL balances
				// x/bank balances + x/precisebank balance
				// Exclude "uatom" as x/precisebank balance will include it
				bankCoins := s.network.App.GetBankKeeper().GetAllBalances(s.network.GetContext(), recipientAddr)

				// Get fractional balance after mint
				fracBalAfter := s.network.App.GetPreciseBankKeeper().GetFractionalBalance(s.network.GetContext(), recipientAddr)

				// Only use x/bank balances for non-uatom denoms
				var denoms []string
				for _, coin := range bankCoins {
					// Ignore integer coins, query the extended denom instead
					if coin.Denom == integerDenom {
						continue
					}

					denoms = append(denoms, coin.Denom)
				}

				// Add the extended denom to the list of denoms to balance check
				// Will be included in balance check even if x/bank doesn't have
				// uatom.
				denoms = append(denoms, extendedDenom)

				// All balance queries through x/precisebank
				afterBalance := sdk.NewCoins()
				for _, denom := range denoms {
					coin := s.network.App.GetPreciseBankKeeper().GetBalance(s.network.GetContext(), recipientAddr, denom)
					afterBalance = afterBalance.Add(coin)
				}

				s.Require().Equal(
					mt.wantBalance.String(),
					afterBalance.String(),
					"unexpected balance after minting %s to %s",
				)

				// Check fractinoal balance change event
				if !fracBalAfter.Sub(fracBalBefore).Equal(sdkmath.ZeroInt()) {
					expEvent := types.NewEventFractionalBalanceChange(recipientAddr, fracBalBefore, fracBalAfter)
					events := s.network.GetContext().EventManager().Events()
					s.Require().Contains(events, expEvent)
				}
			}
		})
	}
}

func (s *KeeperIntegrationTestSuite) TestMintCoinsRandomValueMultiDecimals() {
	extendedDenom := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.GetExtendedDenom()
	extendedDecimals := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.ExtendedDecimals

	tests := []struct {
		name        string
		chainConfig testconfig.ChainConfig
	}{
		{
			name:        "6 decimals",
			chainConfig: testconfig.SixDecimalsChainConfig,
		},
		{
			name:        "12 decimals",
			chainConfig: testconfig.TwelveDecimalsChainConfig,
		},
		{
			name:        "2 decimals",
			chainConfig: testconfig.TwoDecimalsChainConfig,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTestWithChainConfig(tt.chainConfig)

			// Has mint permissions
			minterModuleName := evmtypes.ModuleName
			minter := sdk.AccAddress([]byte{1})

			// Target balance
			targetBalance := types.ConversionFactor(extendedDecimals).MulRaw(100)

			// Setup test parameters
			maxMintUnit := types.ConversionFactor(extendedDecimals).MulRaw(2).SubRaw(1)
			r := rand.New(rand.NewSource(SEED))

			totalMinted := sdkmath.ZeroInt()
			mintCount := 0

			// Continue mints as long as target balance is not reached
			for {
				// Check current minter balance
				minterBal := s.GetAllBalances(minter).AmountOf(extendedDenom)
				if minterBal.GTE(targetBalance) {
					break
				}

				// Generate random amount within the range of max possible mint amount
				remaining := targetBalance.Sub(minterBal)
				maxPossible := sdkmath.MinInt(maxMintUnit, remaining)
				randAmount := sdkmath.NewIntFromBigInt(new(big.Int).Rand(r, maxPossible.BigInt())).AddRaw(1)

				// 1. mint to evm module
				mintCoins := cs(ci(extendedDenom, randAmount))
				err := s.network.App.GetPreciseBankKeeper().MintCoins(s.network.GetContext(), minterModuleName, mintCoins)
				s.Require().NoError(err)

				// 2. send to account
				err = s.network.App.GetPreciseBankKeeper().SendCoinsFromModuleToAccount(s.network.GetContext(), minterModuleName, minter, mintCoins)
				s.Require().NoError(err)

				totalMinted = totalMinted.Add(randAmount)
				mintCount++
			}

			s.T().Logf("Completed %d random mints, total minted: %s", mintCount, totalMinted)

			// Check minter balance
			minterBal := s.GetAllBalances(minter).AmountOf(extendedDenom)
			s.Equal(minterBal.BigInt().Cmp(targetBalance.BigInt()), 0, "minter balance mismatch (expected: %s, actual: %s)", targetBalance, minterBal)

			// Check remainder
			remainder := s.network.App.GetPreciseBankKeeper().GetRemainderAmount(s.network.GetContext())
			s.Equal(remainder.BigInt().Cmp(big.NewInt(0)), 0, "remainder should be zero (expected: %s, actual: %s)", big.NewInt(0), remainder)
		})
	}
}

func FuzzMintCoins(f *testing.F) {
	extendedDecimals := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.ExtendedDecimals
	extendedDenom := testconfig.SixDecimalsChainConfig.EvmConfig.CoinInfo.GetExtendedDenom()

	f.Add(int64(0))
	f.Add(int64(100))
	f.Add(types.ConversionFactor(extendedDecimals).Int64())
	f.Add(types.ConversionFactor(extendedDecimals).QuoRaw(2).Int64())
	f.Add(types.ConversionFactor(extendedDecimals).MulRaw(5).Int64())
	f.Add(types.ConversionFactor(extendedDecimals).MulRaw(2).AddRaw(123948723).Int64())

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

		mintCount := int64(10)

		suite.T().Logf("minting %d %d times", amount, mintCount)

		// Mint 10 times to include mints from non-zero balances
		for i := int64(0); i < mintCount; i++ {
			err := suite.network.App.GetPreciseBankKeeper().MintCoins(
				suite.network.GetContext(),
				evmtypes.ModuleName,
				cs(c(extendedDenom, amount)),
			)
			suite.Require().NoError(err)
		}

		// Check full balances
		recipientAddr := suite.network.App.GetAccountKeeper().GetModuleAddress(evmtypes.ModuleName)
		bal := suite.network.App.GetPreciseBankKeeper().GetBalance(suite.network.GetContext(), recipientAddr, extendedDenom)

		suite.Require().Equalf(
			amount*mintCount,
			bal.Amount.Int64(),
			"unexpected balance after minting %d %d times",
			amount,
			mintCount,
		)
	})
}
