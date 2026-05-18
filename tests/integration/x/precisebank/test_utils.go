package precisebank

import (
	"github.com/cosmos/evm/x/precisebank/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

// MintToAccount mints coins to an account with the x/precisebank methods. This
// must be used when minting extended coins, ie. aatom coins. This depends on
// the methods to be properly tested to be implemented correctly.
func (s *KeeperIntegrationTestSuite) MintToAccount(addr sdk.AccAddress, amt sdk.Coins) {
	accBalancesBefore := s.GetAllBalances(addr)

	err := s.network.App.GetPreciseBankKeeper().MintCoins(s.network.GetContext(), minttypes.ModuleName, amt)
	s.Require().NoError(err)

	err = s.network.App.GetPreciseBankKeeper().SendCoinsFromModuleToAccount(s.network.GetContext(), minttypes.ModuleName, addr, amt)
	s.Require().NoError(err)

	// Double check balances are correctly minted and sent to account
	accBalancesAfter := s.GetAllBalances(addr)

	netIncrease := accBalancesAfter.Sub(accBalancesBefore...)
	s.Require().Equal(ConvertCoinsToExtendedCoinDenom(amt), netIncrease)

	s.T().Logf("minted %s to %s", amt, addr)
}

// MintToModuleAccount mints coins to an account with the x/precisebank methods. This
// must be used when minting extended coins, ie. aatom coins. This depends on
// the methods to be properly tested to be implemented correctly.
func (s *KeeperIntegrationTestSuite) MintToModuleAccount(moduleName string, amt sdk.Coins) {
	moduleAddr := s.network.App.GetAccountKeeper().GetModuleAddress(moduleName)
	accBalancesBefore := s.GetAllBalances(moduleAddr)

	err := s.network.App.GetPreciseBankKeeper().MintCoins(s.network.GetContext(), minttypes.ModuleName, amt)
	s.Require().NoError(err)

	err = s.network.App.GetPreciseBankKeeper().SendCoinsFromModuleToModule(s.network.GetContext(), minttypes.ModuleName, moduleName, amt)
	s.Require().NoError(err)

	// Double check balances are correctly minted and sent to account
	accBalancesAfter := s.GetAllBalances(moduleAddr)

	netIncrease := accBalancesAfter.Sub(accBalancesBefore...)
	s.Require().Equal(ConvertCoinsToExtendedCoinDenom(amt), netIncrease)

	s.T().Logf("minted %s to %s", amt, moduleName)
}

// GetAllBalances returns all the account balances for the given account address.
// This returns the extended coin balance if the account has a non-zero balance,
// WITHOUT the integer coin balance.
func (s *KeeperIntegrationTestSuite) GetAllBalances(addr sdk.AccAddress) sdk.Coins {
	// Get all balances for an account
	bankBalances := s.network.App.GetBankKeeper().GetAllBalances(s.network.GetContext(), addr)

	// Remove integer coins from the balance
	for _, coin := range bankBalances {
		if coin.Denom == types.IntegerCoinDenom() {
			bankBalances = bankBalances.Sub(coin)
		}
	}

	// Replace the integer coin with the extended coin, from x/precisebank
	extendedBal := s.network.App.GetPreciseBankKeeper().GetBalance(s.network.GetContext(), addr, types.ExtendedCoinDenom())

	return bankBalances.Add(extendedBal)
}

// ConvertCoinsToExtendedCoinDenom converts sdk.Coins that includes Integer denoms
// to sdk.Coins that includes Extended denoms of the same amount. This is useful
// for testing to make sure only extended amounts are compared instead of double
// counting balances.
func ConvertCoinsToExtendedCoinDenom(coins sdk.Coins) sdk.Coins {
	integerCoinAmt := coins.AmountOf(types.IntegerCoinDenom())
	if integerCoinAmt.IsZero() {
		return coins
	}

	// Remove the integer coin from the coins
	integerCoin := sdk.NewCoin(types.IntegerCoinDenom(), integerCoinAmt)

	// Add the equivalent extended coin to the coins
	extendedCoin := sdk.NewCoin(types.ExtendedCoinDenom(), integerCoinAmt.Mul(types.ConversionFactor()))

	return coins.Sub(integerCoin).Add(extendedCoin)
}

func c(denom string, amount int64) sdk.Coin        { return sdk.NewInt64Coin(denom, amount) }
func ci(denom string, amount sdkmath.Int) sdk.Coin { return sdk.NewCoin(denom, amount) }
func cs(coins ...sdk.Coin) sdk.Coins               { return sdk.NewCoins(coins...) }
