package precisebank

import (
	"fmt"
	"maps"
	"math/big"
	"math/rand"
	"sort"
	"testing"

	corevm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"

	testconstants "github.com/cosmos/evm/testutil/constants"
	cosmosevmutils "github.com/cosmos/evm/utils"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/cosmos/evm/x/precisebank/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (s *KeeperIntegrationTestSuite) TestSendCoinsFromAccountToModuleMatchingErrors() {
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
			cs(c(types.ExtendedCoinDenom(), 1000)),
			"module account cat does not exist: unknown address",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// Reset
			s.SetupTest()

			s.Require().NotEmpty(tt.wantPanic, "test case must have a wantPanic")

			s.Require().PanicsWithError(tt.wantPanic, func() {
				err := s.network.App.GetBankKeeper().SendCoinsFromAccountToModule(s.network.GetContext(), tt.sender, tt.recipientModule, tt.sendAmount)
				s.Require().Error(err)
			}, "wantPanic should match x/bank SendCoinsFromAccountToModule panic")

			s.Require().PanicsWithError(tt.wantPanic, func() {
				err := s.network.App.GetPreciseBankKeeper().SendCoinsFromAccountToModule(s.network.GetContext(), tt.sender, tt.recipientModule, tt.sendAmount)
				s.Require().Error(err)
			}, "x/precisebank panic should match x/bank SendCoinsFromAccountToModule panic")
		})
	}
}

func (s *KeeperIntegrationTestSuite) TestSendCoinsFromModuleToAccountMatchingErrors() {
	// Ensure errors match x/bank errors AND panics. This needs to be well
	// tested before SendCoins as all send tests rely on this to initialize
	// account balances.
	// No unit test with mock x/bank for SendCoinsFromModuleToAccount since
	// we only are testing the errors/panics specific to the method and
	// remaining logic is the same as SendCoins.

	blockedMacAddrs := blockedAddresses()
	precisebankAddr := s.network.App.GetAccountKeeper().GetModuleAddress(types.ModuleName)

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
	macPerms := getMaccPerms()
	for moduleName := range macPerms {
		if moduleName != types.ModuleName && moduleName != stakingtypes.BondedPoolName {
			senderModuleName = moduleName
		}
	}

	s.Require().NotEmpty(blockedAddr, "no blocked addresses found")
	s.Require().NotEmpty(senderModuleName, "no sender module name found")

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
			cs(c(types.ExtendedCoinDenom(), 1000)),
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
			cs(c(types.ExtendedCoinDenom(), 1000)),
			fmt.Sprintf("%s is not allowed to receive funds: unauthorized", blockedAddr.String()),
			"",
		},
		// SendCoins specific errors/panics
		{
			"invalid coins",
			senderModuleName,
			sdk.AccAddress([]byte{2}),
			sdk.Coins{sdk.Coin{Denom: types.IntegerCoinDenom(), Amount: sdkmath.NewInt(-1)}},
			fmt.Sprintf("-1%s: invalid coins", types.IntegerCoinDenom()),
			"",
		},
		{
			"insufficient balance - passthrough",
			senderModuleName,
			sdk.AccAddress([]byte{2}),
			cs(c(types.IntegerCoinDenom(), 1000)),
			fmt.Sprintf("spendable balance 0%s is smaller than 1000%s: insufficient funds",
				types.IntegerCoinDenom(), types.IntegerCoinDenom()),
			"",
		},
		{
			"insufficient balance - extended",
			senderModuleName,
			sdk.AccAddress([]byte{2}),
			// We can still test insufficient bal errors with "aatom" since
			// we also expect it to not exist in x/bank
			cs(c(types.ExtendedCoinDenom(), 1000)),
			fmt.Sprintf("spendable balance 0%s is smaller than 1000%s: insufficient funds",
				types.ExtendedCoinDenom(), types.ExtendedCoinDenom()),
			"",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// Reset
			s.SetupTest()

			if tt.wantPanic == "" && tt.wantErr == "" {
				s.FailNow("test case must have a wantErr or wantPanic")
			}

			if tt.wantPanic != "" {
				s.Require().Empty(tt.wantErr, "test case must not have a wantErr if wantPanic is set")

				s.Require().PanicsWithError(tt.wantPanic, func() {
					err := s.network.App.GetBankKeeper().SendCoinsFromModuleToAccount(s.network.GetContext(), tt.senderModule, tt.recipient, tt.sendAmount)
					s.Require().Error(err)
				}, "wantPanic should match x/bank SendCoinsFromModuleToAccount panic")

				s.Require().PanicsWithError(tt.wantPanic, func() {
					err := s.network.App.GetPreciseBankKeeper().SendCoinsFromModuleToAccount(s.network.GetContext(), tt.senderModule, tt.recipient, tt.sendAmount)
					s.Require().Error(err)
				}, "x/precisebank panic should match x/bank SendCoinsFromModuleToAccount panic")
			}

			if tt.wantErr != "" {
				bankErr := s.network.App.GetBankKeeper().SendCoinsFromModuleToAccount(s.network.GetContext(), tt.senderModule, tt.recipient, tt.sendAmount)
				s.Require().Error(bankErr)
				s.Require().EqualError(bankErr, tt.wantErr, "expected error should match x/bank SendCoins error")

				pbankErr := s.network.App.GetPreciseBankKeeper().SendCoinsFromModuleToAccount(s.network.GetContext(), tt.senderModule, tt.recipient, tt.sendAmount)
				s.Require().Error(pbankErr)
				// Compare strings instead of errors, as error stack is still different
				s.Require().Equal(
					bankErr.Error(),
					pbankErr.Error(),
					"x/precisebank error should match x/bank SendCoins error",
				)
			}
		})
	}
}

func (s *KeeperIntegrationTestSuite) TestSendCoinsMatchingErrors() {
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
			sdk.Coins{sdk.Coin{Denom: types.IntegerCoinDenom(), Amount: sdkmath.NewInt(-1)}},
			fmt.Sprintf("-1%s: invalid coins",
				types.IntegerCoinDenom()),
		},
		{
			"insufficient empty balance - passthrough",
			cs(),
			cs(c(types.IntegerCoinDenom(), 1000)),
			fmt.Sprintf("spendable balance 0%s is smaller than 1000%s: insufficient funds",
				types.IntegerCoinDenom(), types.IntegerCoinDenom()),
		},
		{
			"insufficient empty balance - extended",
			cs(),
			// We can still test insufficient bal errors with "aatom" since
			// we also expect it to not exist in x/bank
			cs(c(types.ExtendedCoinDenom(), 1000)),
			fmt.Sprintf("spendable balance 0%s is smaller than 1000%s: insufficient funds",
				types.ExtendedCoinDenom(), types.ExtendedCoinDenom()),
		},
		{
			"insufficient non-empty balance - passthrough",
			cs(c(types.IntegerCoinDenom(), 100), c("usdc", 1000)),
			cs(c(types.IntegerCoinDenom(), 1000)),
			fmt.Sprintf("spendable balance 100%s is smaller than 1000%s: insufficient funds",
				types.IntegerCoinDenom(), types.IntegerCoinDenom()),
		},
		// non-empty aatom transfer error is tested in SendCoins, not here since
		// x/bank doesn't hold aatom
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			// Reset
			s.SetupTest()
			sender := sdk.AccAddress([]byte{1})
			recipient := sdk.AccAddress([]byte{2})

			s.Require().NotEmpty(tt.wantErr, "test case must have a wantErr")

			s.MintToAccount(sender, tt.initialAmount)

			bankErr := s.network.App.GetBankKeeper().SendCoins(s.network.GetContext(), sender, recipient, tt.sendAmount)
			s.Require().Error(bankErr)
			s.Require().EqualError(bankErr, tt.wantErr, "expected error should match x/bank SendCoins error")

			pbankErr := s.network.App.GetPreciseBankKeeper().SendCoins(s.network.GetContext(), sender, recipient, tt.sendAmount)
			s.Require().Error(pbankErr)
			// Compare strings instead of errors, as error stack is still different
			s.Require().Equal(
				bankErr.Error(),
				pbankErr.Error(),
				"x/precisebank error should match x/bank SendCoins error",
			)
		})
	}
}

func (s *KeeperIntegrationTestSuite) TestSendCoins() {
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
			cs(c(types.ExtendedCoinDenom(), 10), c("usdc", 1000)),
			cs(),
			cs(c(types.ExtendedCoinDenom(), 1000)),
			fmt.Sprintf("spendable balance 10%s is smaller than 1000%s: insufficient funds",
				types.ExtendedCoinDenom(), types.ExtendedCoinDenom()),
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
			cs(c(types.IntegerCoinDenom(), 1000)),
			cs(),
			cs(c(types.IntegerCoinDenom(), 1000)),
			"",
		},
		{
			"passthrough & extended",
			cs(c(types.IntegerCoinDenom(), 1000)),
			cs(),
			cs(c(types.IntegerCoinDenom(), 10), c(types.ExtendedCoinDenom(), 1)),
			"",
		},
		{
			"aatom send - 1aatom to 0 balance",
			// Starting balances
			cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor().MulRaw(5))),
			cs(),
			// Send amount
			cs(c(types.ExtendedCoinDenom(), 1)), // aatom
			"",
		},
		{
			"sender borrow from integer",
			// 1uatom, 0 fractional
			cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor())),
			cs(),
			// Send 1 with 0 fractional balance
			cs(c(types.ExtendedCoinDenom(), 1)),
			"",
		},
		{
			"sender borrow from integer - max fractional amount",
			// 1uatom, 0 fractional
			cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor())),
			cs(),
			// Max fractional amount
			cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor().SubRaw(1))),
			"",
		},
		{
			"receiver carry",
			cs(c(types.ExtendedCoinDenom(), 1000)),
			// max fractional amount, carries over to integer
			cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor().SubRaw(1))),
			cs(c(types.ExtendedCoinDenom(), 1)),
			"",
		},
		{
			"receiver carry - max fractional amount",
			cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor().MulRaw(5))),
			// max fractional amount, carries over to integer
			cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor().SubRaw(1))),
			cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor().SubRaw(1))),
			"",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			sender := sdk.AccAddress([]byte{1})
			recipient := sdk.AccAddress([]byte{2})

			// Initialize balances
			s.MintToAccount(sender, tt.giveStartBalSender)
			s.MintToAccount(recipient, tt.giveStartBalRecipient)

			senderBalBefore := s.GetAllBalances(sender)
			recipientBalBefore := s.GetAllBalances(recipient)

			senderFracBalBefore := s.network.App.GetPreciseBankKeeper().GetFractionalBalance(s.network.GetContext(), sender)
			recipientFracBalBefore := s.network.App.GetPreciseBankKeeper().GetFractionalBalance(s.network.GetContext(), recipient)

			err := s.network.App.GetPreciseBankKeeper().SendCoins(s.network.GetContext(), sender, recipient, tt.giveAmt)
			if tt.wantErr != "" {
				s.Require().Error(err)
				s.Require().EqualError(err, tt.wantErr)
				return
			}

			s.Require().NoError(err)

			// Check balances
			senderBalAfter := s.GetAllBalances(sender)
			recipientBalAfter := s.GetAllBalances(recipient)

			senderFracBalAfter := s.network.App.GetPreciseBankKeeper().GetFractionalBalance(s.network.GetContext(), sender)
			recipientFracBalAfter := s.network.App.GetPreciseBankKeeper().GetFractionalBalance(s.network.GetContext(), recipient)

			// Convert send amount coins to extended coins. i.e. if send coins
			// includes uatom, convert it so that its the equivalent aatom
			// amount so its easier to compare. Compare extended coins only.
			sendAmountFullExtended := tt.giveAmt
			sendAmountInteger := tt.giveAmt.AmountOf(types.IntegerCoinDenom())
			if !sendAmountInteger.IsZero() {
				integerCoin := sdk.NewCoin(types.IntegerCoinDenom(), sendAmountInteger)
				sendAmountFullExtended = sendAmountFullExtended.Sub(integerCoin)

				// Add equivalent extended coin
				extendedCoinAmount := sendAmountInteger.Mul(types.ConversionFactor())
				extendedCoin := sdk.NewCoin(types.ExtendedCoinDenom(), extendedCoinAmount)
				sendAmountFullExtended = sendAmountFullExtended.Add(extendedCoin)
			}

			s.Require().Equal(
				senderBalBefore.Sub(sendAmountFullExtended...),
				senderBalAfter,
			)

			s.Require().Equal(
				recipientBalBefore.Add(sendAmountFullExtended...),
				recipientBalAfter,
			)

			// Check events
			events := s.network.GetContext().EventManager().Events()
			targetEvents := []sdk.Event{}
			for _, event := range events {
				if event.Type == types.EventTypeFractionalBalanceChange {
					targetEvents = append(targetEvents, event)
				}
			}

			if !senderFracBalAfter.Sub(senderFracBalBefore).Equal(sdkmath.ZeroInt()) {
				expSenderFracBalChangeEvent := types.NewEventFractionalBalanceChange(
					sender, senderFracBalBefore, senderFracBalAfter,
				)
				s.Require().Contains(targetEvents, expSenderFracBalChangeEvent)
			}

			if !recipientFracBalAfter.Sub(recipientFracBalBefore).Equal(sdkmath.ZeroInt()) {
				expRecipientFracBalChangeEvent := types.NewEventFractionalBalanceChange(
					recipient, recipientFracBalBefore, recipientFracBalAfter,
				)
				s.Require().Contains(targetEvents, expRecipientFracBalChangeEvent)
			}
		})
	}
}

func (s *KeeperIntegrationTestSuite) TestSendCoinsMatrix() {
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
		{"integer only", cs(c(types.IntegerCoinDenom(), 1000))},
		{"extended only", cs(c(types.ExtendedCoinDenom(), 1000))},
		{"integer & extended", cs(c(types.IntegerCoinDenom(), 1000), c(types.ExtendedCoinDenom(), 1000))},
		{"integer & extended - max fractional", cs(c(types.IntegerCoinDenom(), 1000), ci(types.ExtendedCoinDenom(), types.ConversionFactor().SubRaw(1)))},
		{"integer & extended - min fractional", cs(c(types.IntegerCoinDenom(), 1000), c(types.ExtendedCoinDenom(), 1))},
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
			cs(c(types.IntegerCoinDenom(), 10)),
		},
		{
			"extended only",
			cs(c(types.ExtendedCoinDenom(), 10)),
		},
		{
			"integer & extended",
			cs(c(types.IntegerCoinDenom(), 10), c(types.ExtendedCoinDenom(), 1000)),
		},
		{
			"integer & extended - max fractional",
			cs(c(types.IntegerCoinDenom(), 10), ci(types.ExtendedCoinDenom(), types.ConversionFactor().SubRaw(1))),
		},
		{
			"integer & extended - min fractional",
			cs(c(types.IntegerCoinDenom(), 10), c(types.ExtendedCoinDenom(), 1)),
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

				s.Run(testName, func() {
					s.SetupTest()

					sender := sdk.AccAddress([]byte{1})
					recipient := sdk.AccAddress([]byte{2})

					// Initialize balances
					s.MintToAccount(sender, senderStartBal.bal)
					s.MintToAccount(recipient, recipientStartBal.bal)

					// balances & send amount will only contain total equivalent
					// extended coins and no integer coins so its easier to compare
					senderBalBefore := s.GetAllBalances(sender)
					recipientBalBefore := s.GetAllBalances(recipient)

					sendAmtNormalized := ConvertCoinsToExtendedCoinDenom(sendAmt.amt)

					err := s.network.App.GetPreciseBankKeeper().SendCoins(s.network.GetContext(), sender, recipient, sendAmt.amt)

					hasSufficientBal := senderBalBefore.IsAllGTE(sendAmtNormalized)

					if hasSufficientBal {
						s.Require().NoError(err)
					} else {
						s.Require().Error(err, "expected insufficient funds error")
						// No balance checks if insufficient funds
						return
					}

					// Check balances
					senderBalAfter := s.GetAllBalances(sender)
					recipientBalAfter := s.GetAllBalances(recipient)

					// Convert send amount coins to extended coins. i.e. if send coins
					// includes uatom, convert it so that its the equivalent aatom
					// amount so its easier to compare. Compare extended coins only.

					s.Require().Equal(
						senderBalBefore.Sub(sendAmtNormalized...),
						senderBalAfter,
					)

					s.Require().Equal(
						recipientBalBefore.Add(sendAmtNormalized...),
						recipientBalAfter,
					)
				})
			}
		}
	}
}

func (s *KeeperIntegrationTestSuite) TestSendCoinsFromAccountToModule() {
	// Ensure recipient correctly matches the specified module account. Specific
	// send amount and cases are handled by SendCoins() tests, so we are only
	// checking SendCoinsFromAccountToModule specific behavior here.

	sender := sdk.AccAddress([]byte{1})
	recipientModule := minttypes.ModuleName
	recipientAddr := s.network.App.GetAccountKeeper().GetModuleAddress(recipientModule)

	sendAmt := cs(c(types.ExtendedCoinDenom(), 1000))

	s.MintToAccount(sender, sendAmt)

	err := s.network.App.GetPreciseBankKeeper().SendCoinsFromAccountToModule(
		s.network.GetContext(),
		sender,
		recipientModule,
		sendAmt,
	)
	s.Require().NoError(err)

	// Check balances
	senderBalAfter := s.GetAllBalances(sender)
	recipientBalAfter := s.GetAllBalances(recipientAddr)

	s.Require().Equal(
		cs(),
		senderBalAfter,
	)

	s.Require().Equal(
		sendAmt,
		recipientBalAfter,
	)
}

func (s *KeeperIntegrationTestSuite) TestSendCoinsFromAccountToModuleBlockedRecipientCarry() {
	// Carrying to module account balance. This tests that SendCoinsFromAccountToModule
	// does not fail when sending to a blocked module account.

	sender := sdk.AccAddress([]byte{1})

	sendAmt := cs(c(types.ExtendedCoinDenom(), 1000))
	sendAmt2 := cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor().SubRaw(10)))

	s.MintToAccount(sender, sendAmt.Add(sendAmt2...))

	err := s.network.App.GetPreciseBankKeeper().SendCoinsFromAccountToModule(
		s.network.GetContext(),
		sender,
		authtypes.FeeCollectorName,
		sendAmt,
	)
	s.Require().NoError(err)

	// Trigger carry for fee_collector module account
	err = s.network.App.GetPreciseBankKeeper().SendCoinsFromAccountToModule(
		s.network.GetContext(),
		sender,
		authtypes.FeeCollectorName,
		sendAmt2,
	)
	s.Require().NoError(err)
}

func (s *KeeperIntegrationTestSuite) TestSendCoinsBlockedRecipientCarry() {
	// Same test as TestSendCoinsFromModuleToAccount_Blocked, but with SendCoins
	// which also should not fail when sending to a blocked module account.
	sender := sdk.AccAddress([]byte{1})

	sendAmt := cs(c(types.ExtendedCoinDenom(), 1000))
	sendAmt2 := cs(ci(types.ExtendedCoinDenom(), types.ConversionFactor().SubRaw(10)))

	s.MintToAccount(sender, sendAmt.Add(sendAmt2...))

	recipient := s.network.App.GetAccountKeeper().GetModuleAddress(authtypes.FeeCollectorName)

	err := s.network.App.GetPreciseBankKeeper().SendCoins(
		s.network.GetContext(),
		sender,
		recipient,
		sendAmt,
	)
	s.Require().NoError(err)

	// Trigger carry for fee_collector module account
	err = s.network.App.GetPreciseBankKeeper().SendCoins(
		s.network.GetContext(),
		sender,
		recipient,
		sendAmt2,
	)
	s.Require().NoError(err)
}

func (s *KeeperIntegrationTestSuite) TestSendCoinsFromModuleToAccount() {
	// Ensure sender correctly matches the specified module account. Opposite
	// of SendCoinsFromAccountToModule, so we are only checking the correct
	// addresses are being used.

	senderModule := evmtypes.ModuleName
	senderAddr := s.network.App.GetAccountKeeper().GetModuleAddress(senderModule)

	recipient := sdk.AccAddress([]byte{1})

	sendAmt := cs(c(types.ExtendedCoinDenom(), 1000))

	s.MintToModuleAccount(senderModule, sendAmt)

	err := s.network.App.GetPreciseBankKeeper().SendCoinsFromModuleToAccount(
		s.network.GetContext(),
		senderModule,
		recipient,
		sendAmt,
	)
	s.Require().NoError(err)

	// Check balances
	senderBalAfter := s.GetAllBalances(senderAddr)
	recipientBalAfter := s.GetAllBalances(recipient)

	s.Require().Equal(
		cs(),
		senderBalAfter,
	)

	s.Require().Equal(
		sendAmt,
		recipientBalAfter,
	)
}

func (s *KeeperIntegrationTestSuite) TestSendCoinsRandomValueMultiDecimals() {
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
		s.Run(tt.name, func() {
			s.SetupTestWithChainID(tt.chainID)

			sender := sdk.AccAddress([]byte{1})
			recipient := sdk.AccAddress([]byte{2})

			// Initial balance large enough to cover many small sends
			initialBalance := types.ConversionFactor().MulRaw(100)
			s.MintToAccount(sender, cs(ci(types.ExtendedCoinDenom(), initialBalance)))

			// Setup test parameters
			maxSendUnit := types.ConversionFactor().MulRaw(2).SubRaw(1)
			r := rand.New(rand.NewSource(SEED))

			totalSent := sdkmath.ZeroInt()
			sentCount := 0

			// Continue transfers as long as sender has balance remaining
			for {
				// Check current sender balance
				senderAmount := s.GetAllBalances(sender).AmountOf(types.ExtendedCoinDenom())
				if senderAmount.IsZero() {
					break
				}

				// Generate random amount within the range of max possible send amount
				maxPossibleSend := maxSendUnit
				if maxPossibleSend.GT(senderAmount) {
					maxPossibleSend = senderAmount
				}
				randAmount := sdkmath.NewIntFromBigInt(new(big.Int).Rand(r, maxPossibleSend.BigInt())).AddRaw(1)

				sendAmount := cs(ci(types.ExtendedCoinDenom(), randAmount))
				err := s.network.App.GetPreciseBankKeeper().SendCoins(s.network.GetContext(), sender, recipient, sendAmount)
				s.NoError(err)
				totalSent = totalSent.Add(randAmount)
				sentCount++
			}

			s.T().Logf("Completed %d random sends, total sent: %s", sentCount, totalSent.String())

			// Check sender balance
			senderAmount := s.GetAllBalances(sender).AmountOf(types.ExtendedCoinDenom())
			s.Equal(senderAmount.BigInt().Cmp(big.NewInt(0)), 0, "sender balance should be zero")

			// Check recipient balance
			recipientBal := s.GetAllBalances(recipient)
			intReceived := recipientBal.AmountOf(types.ExtendedCoinDenom()).Quo(types.ConversionFactor())
			fracReceived := s.network.App.GetPreciseBankKeeper().GetFractionalBalance(s.network.GetContext(), recipient)

			expectedInt := totalSent.Quo(types.ConversionFactor())
			expectedFrac := totalSent.Mod(types.ConversionFactor())

			s.Equal(expectedInt.BigInt().Cmp(intReceived.BigInt()), 0, "integer carry mismatch (expected: %s, received: %s)", expectedInt, intReceived)
			s.Equal(expectedFrac.BigInt().Cmp(fracReceived.BigInt()), 0, "fractional balance mismatch (expected: %s, received: %s)", expectedFrac, fracReceived)
		})
	}
}

func FuzzSendCoins(f *testing.F) {
	configurator := evmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	configurator.WithEVMCoinInfo(testconstants.ExampleChainCoinInfo[testconstants.SixDecimalsChainID])
	err := configurator.Configure()
	require.NoError(f, err)

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
		suite.MintToAccount(sender, cs(c(types.ExtendedCoinDenom(), int64(startBalSender))))      //nolint:gosec // G115
		suite.MintToAccount(recipient, cs(c(types.ExtendedCoinDenom(), int64(startBalReceiver)))) //nolint:gosec // G115

		// Send amount
		sendCoins := cs(c(types.ExtendedCoinDenom(), int64(sendAmount))) //nolint:gosec // G115
		err := suite.network.App.GetPreciseBankKeeper().SendCoins(suite.network.GetContext(), sender, recipient, sendCoins)
		if startBalSender < sendAmount {
			suite.Require().Error(err, "expected insufficient funds error")
			return
		}

		suite.Require().NoError(err)

		// Check full balances
		balSender := suite.GetAllBalances(sender)
		balReceiver := suite.GetAllBalances(recipient)

		suite.Require().Equal(
			startBalSender-sendAmount,
			balSender.AmountOf(types.ExtendedCoinDenom()).Uint64(),
		)
		suite.Require().Equal(
			startBalReceiver+sendAmount,
			balReceiver.AmountOf(types.ExtendedCoinDenom()).Uint64(),
		)
	})
}

func (s *KeeperIntegrationTestSuite) TestSendMsg_RandomValueMultiDecimals() { //nolint:revive // false positive due to file name
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
		s.Run(tt.name, func() {
			s.SetupTestWithChainID(tt.chainID)

			sender := sdk.AccAddress([]byte{1})
			recipient := sdk.AccAddress([]byte{2})

			// Initial balance large enough to cover many small sends
			initialBalance := types.ConversionFactor().MulRaw(100)
			s.MintToAccount(sender, cs(ci(types.ExtendedCoinDenom(), initialBalance)))

			// Setup test parameters
			maxSendUnit := types.ConversionFactor().MulRaw(2).SubRaw(1)
			r := rand.New(rand.NewSource(SEED))

			totalSent := sdkmath.ZeroInt()
			sentCount := 0

			// Continue transfers as long as sender has balance remaining
			for {
				// Check current sender balance
				senderAmount := s.GetAllBalances(sender).AmountOf(types.ExtendedCoinDenom())
				if senderAmount.IsZero() {
					break
				}

				// Generate random amount within the range of max possible send amount
				maxPossibleSend := maxSendUnit
				if maxPossibleSend.GT(senderAmount) {
					maxPossibleSend = senderAmount
				}
				randAmount := sdkmath.NewIntFromBigInt(new(big.Int).Rand(r, maxPossibleSend.BigInt())).AddRaw(1)

				sendAmount := cs(ci(types.ExtendedCoinDenom(), randAmount))
				msgSend := banktypes.MsgSend{
					FromAddress: sender.String(),
					ToAddress:   recipient.String(),
					Amount:      sendAmount,
				}
				_, err := s.network.App.GetPreciseBankKeeper().Send(s.network.GetContext(), &msgSend)
				s.NoError(err)
				totalSent = totalSent.Add(randAmount)
				sentCount++
			}

			s.T().Logf("Completed %d random sends, total sent: %s", sentCount, totalSent.String())

			// Check sender balance
			senderAmount := s.GetAllBalances(sender).AmountOf(types.ExtendedCoinDenom())
			s.Equal(senderAmount.BigInt().Cmp(big.NewInt(0)), 0, "sender balance should be zero")

			// Check recipient balance
			recipientBal := s.GetAllBalances(recipient)
			intReceived := recipientBal.AmountOf(types.ExtendedCoinDenom()).Quo(types.ConversionFactor())
			fracReceived := s.network.App.GetPreciseBankKeeper().GetFractionalBalance(s.network.GetContext(), recipient)

			expectedInt := totalSent.Quo(types.ConversionFactor())
			expectedFrac := totalSent.Mod(types.ConversionFactor())

			s.Equal(expectedInt.BigInt().Cmp(intReceived.BigInt()), 0, "integer carry mismatch (expected: %s, received: %s)", expectedInt, intReceived)
			s.Equal(expectedFrac.BigInt().Cmp(fracReceived.BigInt()), 0, "fractional balance mismatch (expected: %s, received: %s)", expectedFrac, fracReceived)
		})
	}
}

func blockedAddresses() map[string]bool {
	blockedAddrs := make(map[string]bool)

	maccPerms := getMaccPerms()
	accs := make([]string, 0, len(maccPerms))
	for acc := range maccPerms {
		accs = append(accs, acc)
	}
	sort.Strings(accs)

	for _, acc := range accs {
		blockedAddrs[authtypes.NewModuleAddress(acc).String()] = true
	}

	blockedPrecompilesHex := evmtypes.AvailableStaticPrecompiles
	for _, addr := range corevm.PrecompiledAddressesPrague {
		blockedPrecompilesHex = append(blockedPrecompilesHex, addr.Hex())
	}

	for _, precompile := range blockedPrecompilesHex {
		blockedAddrs[cosmosevmutils.Bech32StringFromHexAddress(precompile)] = true
	}

	return blockedAddrs
}

// module account permissions
var maccPerms = map[string][]string{
	authtypes.FeeCollectorName:     nil,
	distrtypes.ModuleName:          nil,
	ibctransfertypes.ModuleName:    {authtypes.Minter, authtypes.Burner},
	minttypes.ModuleName:           {authtypes.Minter},
	stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
	stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
	govtypes.ModuleName:            {authtypes.Burner},

	// Cosmos EVM modules
	evmtypes.ModuleName:         {authtypes.Minter, authtypes.Burner},
	feemarkettypes.ModuleName:   nil,
	erc20types.ModuleName:       {authtypes.Minter, authtypes.Burner},
	precisebanktypes.ModuleName: {authtypes.Minter, authtypes.Burner},
}

// getMaccPerms returns a copy of the module account permissions
func getMaccPerms() map[string][]string {
	return maps.Clone(maccPerms)
}
