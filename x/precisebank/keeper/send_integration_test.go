package keeper_test

import (
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/x/precisebank/keeper"
	"github.com/cosmos/evm/x/precisebank/types"
)

func (suite *KeeperIntegrationTestSuite) TestSendCoinsFromAccountToModule_MatchingErrors() {
	// No specific errors for SendCoinsFromAccountToModule, only 1 panic if
	// the module account does not exist

	tests := []struct {
		name            string
		sender          sdk.AccAddress
		recipientModule string
		sendAmount      sdk.Coins
		wantPanic       string
	}{
		// SendCoinsFromAccountToModule specific errors/panics
		{
			"missing module account - passthrough",
			sdk.AccAddress([]byte{2}),
			"cat",
			cs(c("usdc", 1000)),
			"module account cat does not exist: unknown address",
		},
		{
			"missing module account - extended",
			sdk.AccAddress([]byte{2}),
			"cat",
			cs(c(types.ExtendedCoinDenom, 1000)),
			"module account cat does not exist: unknown address",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset
			suite.SetupTest()

			suite.Require().NotEmpty(tt.wantPanic, "test case must have a wantPanic")

			suite.Require().PanicsWithError(tt.wantPanic, func() {
				suite.network.App.BankKeeper.SendCoinsFromAccountToModule(suite.network.GetContext(), tt.sender, tt.recipientModule, tt.sendAmount)
			}, "wantPanic should match x/bank SendCoinsFromAccountToModule panic")

			suite.Require().PanicsWithError(tt.wantPanic, func() {
				suite.network.App.PreciseBankKeeper.SendCoinsFromAccountToModule(suite.network.GetContext(), tt.sender, tt.recipientModule, tt.sendAmount)
			}, "x/precisebank panic should match x/bank SendCoinsFromAccountToModule panic")
		})
	}
}

func (suite *KeeperIntegrationTestSuite) TestSendCoinsFromModuleToAccount_MatchingErrors() {
	// Ensure errors match x/bank errors AND panics. This needs to be well
	// tested before SendCoins as all send tests rely on this to initialize
	// account balances.
	// No unit test with mock x/bank for SendCoinsFromModuleToAccount since
	// we only are testing the errors/panics specific to the method and
	// remaining logic is the same as SendCoins.

	blockedMacAddrs := evmd.BlockedAddresses()
	precisebankAddr := suite.network.App.AccountKeeper.GetModuleAddress(types.ModuleName)

	var blockedAddr sdk.AccAddress
	// Get the first blocked address
	for addr, isBlocked := range blockedMacAddrs {
		// Skip x/precisebank module account
		if addr == precisebankAddr.String() {
			continue
		}

		if isBlocked {
			blockedAddr = sdk.MustAccAddressFromBech32(addr)
			break
		}
	}

	// We need a ModuleName of another module account to send funds from.
	// x/precisebank is blocked from use with SendCoinsFromModuleToAccount as we
	// don't want external modules to modify x/precisebank balances.
	var senderModuleName string
	macPerms := evmd.GetMaccPerms()
	for moduleName := range macPerms {
		if moduleName != types.ModuleName {
			senderModuleName = moduleName
		}
	}

	suite.Require().NotEmpty(blockedAddr, "no blocked addresses found")
	suite.Require().NotEmpty(senderModuleName, "no sender module name found")

	tests := []struct {
		name         string
		senderModule string
		recipient    sdk.AccAddress
		sendAmount   sdk.Coins
		wantErr      string
		wantPanic    string
	}{
		// SendCoinsFromModuleToAccount specific errors/panics
		{
			"missing module account - passthrough",
			"cat",
			sdk.AccAddress([]byte{2}),
			cs(c("usdc", 1000)),
			"",
			"module account cat does not exist: unknown address",
		},
		{
			"missing module account - extended",
			"cat",
			sdk.AccAddress([]byte{2}),
			cs(c(types.ExtendedCoinDenom, 1000)),
			"",
			"module account cat does not exist: unknown address",
		},
		{
			"blocked recipient address - passthrough",
			senderModuleName,
			blockedAddr,
			cs(c("usdc", 1000)),
			fmt.Sprintf("%s is not allowed to receive funds: unauthorized", blockedAddr.String()),
			"",
		},
		{
			"blocked recipient address - extended",
			senderModuleName,
			blockedAddr,
			cs(c(types.ExtendedCoinDenom, 1000)),
			fmt.Sprintf("%s is not allowed to receive funds: unauthorized", blockedAddr.String()),
			"",
		},
		// SendCoins specific errors/panics
		{
			"invalid coins",
			senderModuleName,
			sdk.AccAddress([]byte{2}),
			sdk.Coins{sdk.Coin{Denom: "uatom", Amount: sdkmath.NewInt(-1)}},
			"-1uatom: invalid coins",
			"",
		},
		{
			"insufficient balance - passthrough",
			senderModuleName,
			sdk.AccAddress([]byte{2}),
			cs(c(types.IntegerCoinDenom, 1000)),
			"spendable balance  is smaller than 1000uatom: insufficient funds",
			"",
		},
		{
			"insufficient balance - extended",
			senderModuleName,
			sdk.AccAddress([]byte{2}),
			// We can still test insufficient bal errors with "aatom" since
			// we also expect it to not exist in x/bank
			cs(c(types.ExtendedCoinDenom, 1000)),
			"spendable balance  is smaller than 1000aatom: insufficient funds",
			"",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset
			suite.SetupTest()

			if tt.wantPanic == "" && tt.wantErr == "" {
				suite.FailNow("test case must have a wantErr or wantPanic")
			}

			if tt.wantPanic != "" {
				suite.Require().Empty(tt.wantErr, "test case must not have a wantErr if wantPanic is set")

				suite.Require().PanicsWithError(tt.wantPanic, func() {
					suite.network.App.BankKeeper.SendCoinsFromModuleToAccount(suite.network.GetContext(), tt.senderModule, tt.recipient, tt.sendAmount)
				}, "wantPanic should match x/bank SendCoinsFromModuleToAccount panic")

				suite.Require().PanicsWithError(tt.wantPanic, func() {
					suite.network.App.PreciseBankKeeper.SendCoinsFromModuleToAccount(suite.network.GetContext(), tt.senderModule, tt.recipient, tt.sendAmount)
				}, "x/precisebank panic should match x/bank SendCoinsFromModuleToAccount panic")
			}

			if tt.wantErr != "" {
				bankErr := suite.network.App.BankKeeper.SendCoinsFromModuleToAccount(suite.network.GetContext(), tt.senderModule, tt.recipient, tt.sendAmount)
				suite.Require().Error(bankErr)
				suite.Require().EqualError(bankErr, tt.wantErr, "expected error should match x/bank SendCoins error")

				pbankErr := suite.network.App.PreciseBankKeeper.SendCoinsFromModuleToAccount(suite.network.GetContext(), tt.senderModule, tt.recipient, tt.sendAmount)
				suite.Require().Error(pbankErr)
				// Compare strings instead of errors, as error stack is still different
				suite.Require().Equal(
					bankErr.Error(),
					pbankErr.Error(),
					"x/precisebank error should match x/bank SendCoins error",
				)
			}
		})
	}
}

func (suite *KeeperIntegrationTestSuite) TestSendCoins_MatchingErrors() {
	// Ensure errors match x/bank errors

	tests := []struct {
		name          string
		initialAmount sdk.Coins
		sendAmount    sdk.Coins
		wantErr       string
	}{
		{
			"invalid coins",
			cs(),
			sdk.Coins{sdk.Coin{Denom: "uatom", Amount: sdkmath.NewInt(-1)}},
			"-1uatom: invalid coins",
		},
		{
			"insufficient empty balance - passthrough",
			cs(),
			cs(c(types.IntegerCoinDenom, 1000)),
			"spendable balance  is smaller than 1000uatom: insufficient funds",
		},
		{
			"insufficient empty balance - extended",
			cs(),
			// We can still test insufficient bal errors with "aatom" since
			// we also expect it to not exist in x/bank
			cs(c(types.ExtendedCoinDenom, 1000)),
			"spendable balance  is smaller than 1000aatom: insufficient funds",
		},
		{
			"insufficient non-empty balance - passthrough",
			cs(c(types.IntegerCoinDenom, 100), c("usdc", 1000)),
			cs(c(types.IntegerCoinDenom, 1000)),
			"spendable balance 100uatom is smaller than 1000uatom: insufficient funds",
		},
		// non-empty aatom transfer error is tested in SendCoins, not here since
		// x/bank doesn't hold aatom
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset
			suite.SetupTest()
			sender := sdk.AccAddress([]byte{1})
			recipient := sdk.AccAddress([]byte{2})

			suite.Require().NotEmpty(tt.wantErr, "test case must have a wantErr")

			suite.MintToAccount(sender, tt.initialAmount)

			bankErr := suite.network.App.BankKeeper.SendCoins(suite.network.GetContext(), sender, recipient, tt.sendAmount)
			suite.Require().Error(bankErr)
			suite.Require().EqualError(bankErr, tt.wantErr, "expected error should match x/bank SendCoins error")

			pbankErr := suite.network.App.PreciseBankKeeper.SendCoins(suite.network.GetContext(), sender, recipient, tt.sendAmount)
			suite.Require().Error(pbankErr)
			// Compare strings instead of errors, as error stack is still different
			suite.Require().Equal(
				bankErr.Error(),
				pbankErr.Error(),
				"x/precisebank error should match x/bank SendCoins error",
			)
		})
	}
}

func (suite *KeeperIntegrationTestSuite) TestSendCoins() {
	// SendCoins is tested mostly in this integration test, as a unit test with
	// mocked BankKeeper overcomplicates expected keepers and makes initializing
	// balances very complex.

	tests := []struct {
		name                  string
		giveStartBalSender    sdk.Coins
		giveStartBalRecipient sdk.Coins
		giveAmt               sdk.Coins
		wantErr               string
	}{
		{
			"insufficient balance error denom matches",
			cs(c(types.ExtendedCoinDenom, 10), c("usdc", 1000)),
			cs(),
			cs(c(types.ExtendedCoinDenom, 1000)),
			"spendable balance 10aatom is smaller than 1000aatom: insufficient funds",
		},
		{
			"passthrough - unrelated",
			cs(c("cats", 1000)),
			cs(),
			cs(c("cats", 1000)),
			"",
		},
		{
			"passthrough - integer denom",
			cs(c(types.IntegerCoinDenom, 1000)),
			cs(),
			cs(c(types.IntegerCoinDenom, 1000)),
			"",
		},
		{
			"passthrough & extended",
			cs(c(types.IntegerCoinDenom, 1000)),
			cs(),
			cs(c(types.IntegerCoinDenom, 10), c(types.ExtendedCoinDenom, 1)),
			"",
		},
		{
			"aatom send - 1aatom to 0 balance",
			// Starting balances
			cs(ci(types.ExtendedCoinDenom, types.ConversionFactor().MulRaw(5))),
			cs(),
			// Send amount
			cs(c(types.ExtendedCoinDenom, 1)), // aatom
			"",
		},
		{
			"sender borrow from integer",
			// 1uatom, 0 fractional
			cs(ci(types.ExtendedCoinDenom, types.ConversionFactor())),
			cs(),
			// Send 1 with 0 fractional balance
			cs(c(types.ExtendedCoinDenom, 1)),
			"",
		},
		{
			"sender borrow from integer - max fractional amount",
			// 1uatom, 0 fractional
			cs(ci(types.ExtendedCoinDenom, types.ConversionFactor())),
			cs(),
			// Max fractional amount
			cs(ci(types.ExtendedCoinDenom, types.ConversionFactor().SubRaw(1))),
			"",
		},
		{
			"receiver carry",
			cs(c(types.ExtendedCoinDenom, 1000)),
			// max fractional amount, carries over to integer
			cs(ci(types.ExtendedCoinDenom, types.ConversionFactor().SubRaw(1))),
			cs(c(types.ExtendedCoinDenom, 1)),
			"",
		},
		{
			"receiver carry - max fractional amount",
			cs(ci(types.ExtendedCoinDenom, types.ConversionFactor().MulRaw(5))),
			// max fractional amount, carries over to integer
			cs(ci(types.ExtendedCoinDenom, types.ConversionFactor().SubRaw(1))),
			cs(ci(types.ExtendedCoinDenom, types.ConversionFactor().SubRaw(1))),
			"",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.SetupTest()

			sender := sdk.AccAddress([]byte{1})
			recipient := sdk.AccAddress([]byte{2})

			// Initialize balances
			suite.MintToAccount(sender, tt.giveStartBalSender)
			suite.MintToAccount(recipient, tt.giveStartBalRecipient)

			senderBalBefore := suite.GetAllBalances(sender)
			recipientBalBefore := suite.GetAllBalances(recipient)

			err := suite.network.App.PreciseBankKeeper.SendCoins(suite.network.GetContext(), sender, recipient, tt.giveAmt)
			if tt.wantErr != "" {
				suite.Require().Error(err)
				suite.Require().EqualError(err, tt.wantErr)
				return
			}

			suite.Require().NoError(err)

			// Check balances
			senderBalAfter := suite.GetAllBalances(sender)
			recipientBalAfter := suite.GetAllBalances(recipient)

			// Convert send amount coins to extended coins. i.e. if send coins
			// includes uatom, convert it so that its the equivalent aatom
			// amount so its easier to compare. Compare extended coins only.
			sendAmountFullExtended := tt.giveAmt
			sendAmountInteger := tt.giveAmt.AmountOf(types.IntegerCoinDenom)
			if !sendAmountInteger.IsZero() {
				integerCoin := sdk.NewCoin(types.IntegerCoinDenom, sendAmountInteger)
				sendAmountFullExtended = sendAmountFullExtended.Sub(integerCoin)

				// Add equivalent extended coin
				extendedCoinAmount := sendAmountInteger.Mul(types.ConversionFactor())
				extendedCoin := sdk.NewCoin(types.ExtendedCoinDenom, extendedCoinAmount)
				sendAmountFullExtended = sendAmountFullExtended.Add(extendedCoin)
			}

			suite.Require().Equal(
				senderBalBefore.Sub(sendAmountFullExtended...),
				senderBalAfter,
			)

			suite.Require().Equal(
				recipientBalBefore.Add(sendAmountFullExtended...),
				recipientBalAfter,
			)

			invariantFn := keeper.AllInvariants(suite.network.App.PreciseBankKeeper)
			res, stop := invariantFn(suite.network.GetContext())
			suite.Require().False(stop, "invariants should not stop")
			suite.Require().Empty(res, "invariants should not return any messages")

			// Check events

			// FULL aatom equivalent, including uatom only/mixed sends
			sendExtendedAmount := sdk.NewCoin(
				types.ExtendedCoinDenom,
				sendAmountFullExtended.AmountOf(types.ExtendedCoinDenom),
			)
			extCoins := sdk.NewCoins(sendExtendedAmount)

			// No extra events if not sending aatom
			if sendExtendedAmount.IsZero() {
				return
			}

			extendedEvent := sdk.NewEvent(
				banktypes.EventTypeTransfer,
				sdk.NewAttribute(banktypes.AttributeKeyRecipient, recipient.String()),
				sdk.NewAttribute(banktypes.AttributeKeySender, sender.String()),
				sdk.NewAttribute(sdk.AttributeKeyAmount, sendExtendedAmount.String()),
			)

			expReceivedEvent := banktypes.NewCoinReceivedEvent(
				recipient,
				extCoins,
			)

			expSentEvent := banktypes.NewCoinSpentEvent(
				sender,
				extCoins,
			)

			events := suite.network.GetContext().EventManager().Events()

			suite.Require().Contains(events, extendedEvent)
			suite.Require().Contains(events, expReceivedEvent)
			suite.Require().Contains(events, expSentEvent)
		})
	}
}

func (suite *KeeperIntegrationTestSuite) TestSendCoins_Matrix() {
	// SendCoins is tested mostly in this integration test, as a unit test with
	// mocked BankKeeper overcomplicates expected keepers and makes initializing
	// balances very complex.

	type startBalance struct {
		name string
		bal  sdk.Coins
	}

	// Run through each combination of start sender/recipient balance & send amt
	// Test matrix fields:
	startBalances := []startBalance{
		{"empty", cs()},
		{"integer only", cs(c(types.IntegerCoinDenom, 1000))},
		{"extended only", cs(c(types.ExtendedCoinDenom, 1000))},
		{"integer & extended", cs(c(types.IntegerCoinDenom, 1000), c(types.ExtendedCoinDenom, 1000))},
		{"integer & extended - max fractional", cs(c(types.IntegerCoinDenom, 1000), ci(types.ExtendedCoinDenom, types.ConversionFactor().SubRaw(1)))},
		{"integer & extended - min fractional", cs(c(types.IntegerCoinDenom, 1000), c(types.ExtendedCoinDenom, 1))},
	}

	sendAmts := []struct {
		name string
		amt  sdk.Coins
	}{
		{
			"empty",
			cs(),
		},
		{
			"integer only",
			cs(c(types.IntegerCoinDenom, 10)),
		},
		{
			"extended only",
			cs(c(types.ExtendedCoinDenom, 10)),
		},
		{
			"integer & extended",
			cs(c(types.IntegerCoinDenom, 10), c(types.ExtendedCoinDenom, 1000)),
		},
		{
			"integer & extended - max fractional",
			cs(c(types.IntegerCoinDenom, 10), ci(types.ExtendedCoinDenom, types.ConversionFactor().SubRaw(1))),
		},
		{
			"integer & extended - min fractional",
			cs(c(types.IntegerCoinDenom, 10), c(types.ExtendedCoinDenom, 1)),
		},
	}

	for _, senderStartBal := range startBalances {
		for _, recipientStartBal := range startBalances {
			for _, sendAmt := range sendAmts {
				testName := fmt.Sprintf(
					"%s -> %s (%s -> %s), send %s (%s)",
					senderStartBal.name, senderStartBal.bal,
					recipientStartBal.name, recipientStartBal.bal,
					sendAmt.name, sendAmt.amt,
				)

				suite.Run(testName, func() {
					suite.SetupTest()

					sender := sdk.AccAddress([]byte{1})
					recipient := sdk.AccAddress([]byte{2})

					// Initialize balances
					suite.MintToAccount(sender, senderStartBal.bal)
					suite.MintToAccount(recipient, recipientStartBal.bal)

					// balances & send amount will only contain total equivalent
					// extended coins and no integer coins so its easier to compare
					senderBalBefore := suite.GetAllBalances(sender)
					recipientBalBefore := suite.GetAllBalances(recipient)

					sendAmtNormalized := ConvertCoinsToExtendedCoinDenom(sendAmt.amt)

					err := suite.network.App.PreciseBankKeeper.SendCoins(suite.network.GetContext(), sender, recipient, sendAmt.amt)

					hasSufficientBal := senderBalBefore.IsAllGTE(sendAmtNormalized)

					if hasSufficientBal {
						suite.Require().NoError(err)
					} else {
						suite.Require().Error(err, "expected insufficient funds error")
						// No balance checks if insufficient funds
						return
					}

					// Check balances
					senderBalAfter := suite.GetAllBalances(sender)
					recipientBalAfter := suite.GetAllBalances(recipient)

					// Convert send amount coins to extended coins. i.e. if send coins
					// includes uatom, convert it so that its the equivalent aatom
					// amount so its easier to compare. Compare extended coins only.

					suite.Require().Equal(
						senderBalBefore.Sub(sendAmtNormalized...),
						senderBalAfter,
					)

					suite.Require().Equal(
						recipientBalBefore.Add(sendAmtNormalized...),
						recipientBalAfter,
					)

					invariantFn := keeper.AllInvariants(suite.network.App.PreciseBankKeeper)
					res, stop := invariantFn(suite.network.GetContext())
					suite.Require().False(stop, "invariants should not stop")
					suite.Require().Empty(res, "invariants should not return any messages")
				})
			}
		}
	}
}

func (suite *KeeperIntegrationTestSuite) TestSendCoinsFromAccountToModule() {
	// Ensure recipient correctly matches the specified module account. Specific
	// send amount and cases are handled by SendCoins() tests, so we are only
	// checking SendCoinsFromAccountToModule specific behavior here.

	sender := sdk.AccAddress([]byte{1})
	recipientModule := minttypes.ModuleName
	recipientAddr := suite.network.App.AccountKeeper.GetModuleAddress(recipientModule)

	sendAmt := cs(c(types.ExtendedCoinDenom, 1000))

	suite.MintToAccount(sender, sendAmt)

	err := suite.network.App.PreciseBankKeeper.SendCoinsFromAccountToModule(
		suite.network.GetContext(),
		sender,
		recipientModule,
		sendAmt,
	)
	suite.Require().NoError(err)

	// Check balances
	senderBalAfter := suite.GetAllBalances(sender)
	recipientBalAfter := suite.GetAllBalances(recipientAddr)

	suite.Require().Equal(
		cs(),
		senderBalAfter,
	)

	suite.Require().Equal(
		sendAmt,
		recipientBalAfter,
	)
}

func (suite *KeeperIntegrationTestSuite) TestSendCoinsFromAccountToModule_BlockedRecipientCarry() {
	// Carrying to module account balance. This tests that SendCoinsFromAccountToModule
	// does not fail when sending to a blocked module account.

	sender := sdk.AccAddress([]byte{1})

	sendAmt := cs(c(types.ExtendedCoinDenom, 1000))
	sendAmt2 := cs(ci(types.ExtendedCoinDenom, types.ConversionFactor().SubRaw(10)))

	suite.MintToAccount(sender, sendAmt.Add(sendAmt2...))

	err := suite.network.App.PreciseBankKeeper.SendCoinsFromAccountToModule(
		suite.network.GetContext(),
		sender,
		authtypes.FeeCollectorName,
		sendAmt,
	)
	suite.Require().NoError(err)

	// Trigger carry for fee_collector module account
	err = suite.network.App.PreciseBankKeeper.SendCoinsFromAccountToModule(
		suite.network.GetContext(),
		sender,
		authtypes.FeeCollectorName,
		sendAmt2,
	)
	suite.Require().NoError(err)
}

func (suite *KeeperIntegrationTestSuite) TestSendCoins_BlockedRecipientCarry() {
	// Same test as TestSendCoinsFromModuleToAccount_Blocked, but with SendCoins
	// which also should not fail when sending to a blocked module account.
	sender := sdk.AccAddress([]byte{1})

	sendAmt := cs(c(types.ExtendedCoinDenom, 1000))
	sendAmt2 := cs(ci(types.ExtendedCoinDenom, types.ConversionFactor().SubRaw(10)))

	suite.MintToAccount(sender, sendAmt.Add(sendAmt2...))

	recipient := suite.network.App.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)

	err := suite.network.App.PreciseBankKeeper.SendCoins(
		suite.network.GetContext(),
		sender,
		recipient,
		sendAmt,
	)
	suite.Require().NoError(err)

	// Trigger carry for fee_collector module account
	err = suite.network.App.PreciseBankKeeper.SendCoins(
		suite.network.GetContext(),
		sender,
		recipient,
		sendAmt2,
	)
	suite.Require().NoError(err)
}

func (suite *KeeperIntegrationTestSuite) TestSendCoinsFromModuleToAccount() {
	// Ensure sender correctly matches the specified module account. Opposite
	// of SendCoinsFromAccountToModule, so we are only checking the correct
	// addresses are being used.

	senderModule := "community"
	senderAddr := suite.network.App.AccountKeeper.GetModuleAddress(senderModule)

	recipient := sdk.AccAddress([]byte{1})

	sendAmt := cs(c(types.ExtendedCoinDenom, 1000))

	suite.MintToAccount(senderAddr, sendAmt)

	err := suite.network.App.PreciseBankKeeper.SendCoinsFromModuleToAccount(
		suite.network.GetContext(),
		senderModule,
		recipient,
		sendAmt,
	)
	suite.Require().NoError(err)

	// Check balances
	senderBalAfter := suite.GetAllBalances(senderAddr)
	recipientBalAfter := suite.GetAllBalances(recipient)

	suite.Require().Equal(
		cs(),
		senderBalAfter,
	)

	suite.Require().Equal(
		sendAmt,
		recipientBalAfter,
	)
}

func FuzzSendCoins(f *testing.F) {
	f.Add(uint64(100), uint64(0), uint64(2))
	f.Add(uint64(100), uint64(100), uint64(5))
	f.Add(types.ConversionFactor().Uint64(), uint64(0), uint64(500))
	f.Add(
		types.ConversionFactor().MulRaw(2).AddRaw(123948723).Uint64(),
		types.ConversionFactor().MulRaw(2).Uint64(),
		types.ConversionFactor().Uint64(),
	)

	f.Fuzz(func(
		t *testing.T,
		startBalSender uint64,
		startBalReceiver uint64,
		sendAmount uint64,
	) {
		// Manually setup test suite since no direct Fuzz support in test suites
		suite := new(KeeperIntegrationTestSuite)
		suite.SetT(t)
		suite.SetS(suite)
		suite.SetupTest()

		sender := sdk.AccAddress([]byte{1})
		recipient := sdk.AccAddress([]byte{2})

		// Initial balances
		suite.MintToAccount(sender, cs(c(types.ExtendedCoinDenom, int64(startBalSender))))
		suite.MintToAccount(recipient, cs(c(types.ExtendedCoinDenom, int64(startBalReceiver))))

		// Send amount
		sendCoins := cs(c(types.ExtendedCoinDenom, int64(sendAmount)))
		err := suite.network.App.PreciseBankKeeper.SendCoins(suite.network.GetContext(), sender, recipient, sendCoins)
		if startBalSender < sendAmount {
			suite.Require().Error(err, "expected insufficient funds error")
			return
		}

		suite.Require().NoError(err)

		// Check FULL balances
		balSender := suite.GetAllBalances(sender)
		balReceiver := suite.GetAllBalances(recipient)

		suite.Require().Equal(
			startBalSender-sendAmount,
			balSender.AmountOf(types.ExtendedCoinDenom).Uint64(),
		)
		suite.Require().Equal(
			startBalReceiver+sendAmount,
			balReceiver.AmountOf(types.ExtendedCoinDenom).Uint64(),
		)

		// Run Invariants to ensure remainder is backing all minted fractions
		// and in a valid state
		allInvariantsFn := keeper.AllInvariants(suite.network.App.PreciseBankKeeper)
		res, stop := allInvariantsFn(suite.network.GetContext())
		suite.Require().False(stop, "invariant should not be broken")
		suite.Require().Empty(res, "unexpected invariant message: %s", res)
	})
}
