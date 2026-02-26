package keeper

import (
	"context"

	"github.com/cosmos/evm/x/precisebank/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetBalance returns the balance of a specific denom for an address. This will
// return the extended balance for the ExtendedCoinDenom, and the regular
// balance for all other denoms.
func (k Keeper) GetBalance(
	goCtx context.Context,
	addr sdk.AccAddress,
	denom string,
) sdk.Coin {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Module balance should display as empty for extended denom. Module
	// balances are **only** for the reserve which backs the fractional
	// balances. Returning the backing balances if querying extended denom would
	// result in a double counting of the fractional balances.
	if denom == types.ExtendedCoinDenom() && addr.Equals(k.ak.GetModuleAddress(types.ModuleName)) {
		return sdk.NewCoin(denom, sdkmath.ZeroInt())
	}

	// Pass through to x/bank for denoms except ExtendedCoinDenom
	if denom != types.ExtendedCoinDenom() {
		return k.bk.GetBalance(ctx, addr, denom)
	}

	// x/bank for integer balance - full balance, including locked
	integerCoins := k.bk.GetBalance(ctx, addr, types.IntegerCoinDenom())

	// x/precisebank for fractional balance
	fractionalAmount := k.GetFractionalBalance(ctx, addr)

	// (Integer * ConversionFactor) + Fractional
	fullAmount := integerCoins.
		Amount.
		Mul(types.ConversionFactor()).
		Add(fractionalAmount)

	return sdk.NewCoin(types.ExtendedCoinDenom(), fullAmount)
}

func (k Keeper) IterateAccountBalances(ctx context.Context, account sdk.AccAddress, cb func(coin sdk.Coin) bool) {
	k.bk.IterateAccountBalances(ctx, account, cb)
}

// SpendableCoins returns the total balances of spendable coins for an account
// by address. If the account has no spendable coins, an empty Coins slice is
// returned.
func (k Keeper) SpendableCoin(
	goCtx context.Context,
	addr sdk.AccAddress,
	denom string,
) sdk.Coin {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Same as GetBalance, extended denom balances are transparent to consumers.
	if denom == types.ExtendedCoinDenom() && addr.Equals(k.ak.GetModuleAddress(types.ModuleName)) {
		return sdk.NewCoin(denom, sdkmath.ZeroInt())
	}

	// Pass through to x/bank for denoms except ExtendedCoinDenom
	if denom != types.ExtendedCoinDenom() {
		return k.bk.SpendableCoin(ctx, addr, denom)
	}

	// x/bank for integer balance - excluding locked
	integerCoin := k.bk.SpendableCoin(ctx, addr, types.IntegerCoinDenom())

	// x/precisebank for fractional balance
	fractionalAmount := k.GetFractionalBalance(ctx, addr)

	// Spendable = (Integer * ConversionFactor) + Fractional
	fullAmount := integerCoin.Amount.
		Mul(types.ConversionFactor()).
		Add(fractionalAmount)

	return sdk.NewCoin(types.ExtendedCoinDenom(), fullAmount)
}
