package wrappers

import (
	"context"
	"fmt"
	"math/big"

	"github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ types.BankWrapper = BankWrapper{}

// BankWrapper is a wrapper around the Cosmos SDK bank keeper
// that is used to manage an evm denom with a custom decimal representation.
type BankWrapper struct {
	types.BankKeeper
}

// NewBankWrapper creates a new BankWrapper instance.
func NewBankWrapper(
	bk types.BankKeeper,
) *BankWrapper {
	return &BankWrapper{
		bk,
	}
}

// ------------------------------------------------------------------------------------------
// Bank wrapper own methods
// ------------------------------------------------------------------------------------------

// MintAmountToAccount converts the given amount into the evm coin scaling
// the amount to the original decimals, then mints that amount to the provided account.
func (w BankWrapper) MintAmountToAccount(ctx context.Context, recipientAddr sdk.AccAddress, amt *big.Int) error {
	coin := sdk.Coin{Denom: types.GetEVMCoinDenom(), Amount: sdkmath.NewIntFromBigInt(amt)}

	convertedCoin, err := types.ConvertEvmCoinDenomToExtendedDenom(coin)
	if err != nil {
		return errors.Wrap(err, "failed to mint coin to account in bank wrapper")
	}

	coinsToMint := sdk.Coins{convertedCoin}
	if err := w.MintCoins(ctx, types.ModuleName, coinsToMint); err != nil {
		return errors.Wrap(err, "failed to mint coins to account in bank wrapper")
	}

	return w.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipientAddr, coinsToMint)
}

// BurnAmountFromAccount converts the given amount into the evm coin scaling
// the amount to the original decimals, then burns that quantity from the provided account.
func (w BankWrapper) BurnAmountFromAccount(ctx context.Context, account sdk.AccAddress, amt *big.Int) error {
	coin := sdk.Coin{Denom: types.GetEVMCoinDenom(), Amount: sdkmath.NewIntFromBigInt(amt)}

	convertedCoin, err := types.ConvertEvmCoinDenomToExtendedDenom(coin)
	if err != nil {
		return errors.Wrap(err, "failed to burn coins from account in bank wrapper")
	}

	coinsToBurn := sdk.Coins{convertedCoin}
	if err := w.BankKeeper.SendCoinsFromAccountToModule(ctx, account, types.ModuleName, coinsToBurn); err != nil {
		return errors.Wrap(err, "failed to burn coins from account in bank wrapper")
	}
	return w.BurnCoins(ctx, types.ModuleName, coinsToBurn)
}

// ------------------------------------------------------------------------------------------
// Bank keeper shadowed methods
// ------------------------------------------------------------------------------------------

// GetBalance returns the balance of the given account.
func (w BankWrapper) GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	if denom != types.GetEVMCoinDenom() {
		panic(fmt.Sprintf("expected evm denom %s, received %s", types.GetEVMCoinDenom(), denom))
	}

	return w.BankKeeper.GetBalance(ctx, addr, types.GetEVMCoinExtendedDenom())
}

// SendCoinsFromAccountToModule wraps around the Cosmos SDK x/bank module's
// SendCoinsFromAccountToModule method to convert the evm coin, if present in
// the input, to its original representation.
func (w BankWrapper) SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, coins sdk.Coins) error {
	convertedCoins := types.ConvertCoinsDenomToExtendedDenom(coins)
	if convertedCoins.IsZero() {
		// if after scaling the coins the amt is zero
		// then is a no-op.
		// Also this avoids getting a validation error on the
		// SendCoinsFromAccountToModule function of the bank keeper
		return nil
	}

	return w.BankKeeper.SendCoinsFromAccountToModule(ctx, senderAddr, recipientModule, convertedCoins)
}

// SendCoinsFromModuleToAccount wraps around the Cosmos SDK x/bank module's
// SendCoinsFromModuleToAccount method to convert the evm coin, if present in
// the input, to its original representation.
func (w BankWrapper) SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, coins sdk.Coins) error {
	convertedCoins := types.ConvertCoinsDenomToExtendedDenom(coins)
	if convertedCoins.IsZero() {
		return nil
	}

	return w.BankKeeper.SendCoinsFromModuleToAccount(ctx, senderModule, recipientAddr, convertedCoins)
}
