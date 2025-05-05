package keeper

import (
	"fmt"

	"github.com/cosmos/evm/x/precisebank/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RegisterInvariants registers the x/precisebank module invariants
func RegisterInvariants(
	ir sdk.InvariantRegistry,
	k Keeper,
) {
	ir.RegisterRoute(types.ModuleName, "reserve-backs-fractions", ReserveBacksFractionsInvariant(k))
	ir.RegisterRoute(types.ModuleName, "balance-remainder-total", BalancedFractionalTotalInvariant(k))
	ir.RegisterRoute(types.ModuleName, "valid-fractional-balances", ValidFractionalAmountsInvariant(k))
	ir.RegisterRoute(types.ModuleName, "valid-remainder-amount", ValidRemainderAmountInvariant(k))
	ir.RegisterRoute(types.ModuleName, "fractional-denom-not-in-bank", FractionalDenomNotInBankInvariant(k))
	ir.RegisterRoute(types.ModuleName, "total-supply", TotalSupplyInvariant(k))
}

// AllInvariants runs all invariants of the X/precisebank module.
func AllInvariants(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		res, stop := ReserveBacksFractionsInvariant(k)(ctx)
		if stop {
			return res, stop
		}

		res, stop = BalancedFractionalTotalInvariant(k)(ctx)
		if stop {
			return res, stop
		}

		res, stop = ValidFractionalAmountsInvariant(k)(ctx)
		if stop {
			return res, stop
		}

		res, stop = ValidRemainderAmountInvariant(k)(ctx)
		if stop {
			return res, stop
		}

		res, stop = FractionalDenomNotInBankInvariant(k)(ctx)
		if stop {
			return res, stop
		}

		res, stop = TotalSupplyInvariant(k)(ctx)
		if stop {
			return res, stop
		}

		return "", false
	}
}

// ReserveBacksFractionsInvariant checks that the total amount of backing
// coins in the reserve is equal to the total amount of fractional balances,
// such that the backing is always available to redeem all fractional balances
// and there are no extra coins in the reserve that are not backing any
// fractional balances.
func ReserveBacksFractionsInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			msg    string
			broken bool
		)

		fractionalBalSum := k.GetTotalSumFractionalBalances(ctx)
		remainderAmount := k.GetRemainderAmount(ctx)

		// Get the total amount of backing coins in the reserve
		moduleAddr := k.ak.GetModuleAddress(types.ModuleName)
		reserveIntegerBalance := k.bk.GetBalance(ctx, moduleAddr, types.IntegerCoinDenom())
		reserveExtendedBalance := reserveIntegerBalance.Amount.Mul(types.ConversionFactor())

		// The total amount of backing coins in the reserve should be equal to
		// fractional balances + remainder amount
		totalRequiredBacking := fractionalBalSum.Add(remainderAmount)

		broken = !reserveExtendedBalance.Equal(totalRequiredBacking)
		msg = fmt.Sprintf(
			"%s reserve balance %s mismatches %s (fractional balances %s + remainder %s)\n",
			types.ExtendedCoinDenom(),
			reserveExtendedBalance,
			totalRequiredBacking,
			fractionalBalSum,
			remainderAmount,
		)

		return sdk.FormatInvariant(
			types.ModuleName, "module reserve backing total fractional balances",
			msg,
		), broken
	}
}

// ValidFractionalAmountsInvariant checks that all individual fractional
// balances are valid.
func ValidFractionalAmountsInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			msg   string
			count int
		)

		k.IterateFractionalBalances(ctx, func(addr sdk.AccAddress, amount sdkmath.Int) bool {
			if err := types.ValidateFractionalAmount(amount); err != nil {
				count++
				msg += fmt.Sprintf("\t%s has an invalid fractional amount of %s\n", addr, amount)
			}

			return false
		})

		broken := count != 0

		return sdk.FormatInvariant(
			types.ModuleName, "valid-fractional-balances",
			fmt.Sprintf("amount of invalid fractional balances found %d\n%s", count, msg),
		), broken
	}
}

// ValidRemainderAmountInvariant checks that the remainder amount is valid.
func ValidRemainderAmountInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var (
			msg    string
			broken bool
		)

		remainderAmount := k.GetRemainderAmount(ctx)

		if !remainderAmount.IsZero() {
			// Only validate if non-zero, as zero is default value
			if err := types.ValidateFractionalAmount(remainderAmount); err != nil {
				broken = true
				msg = fmt.Sprintf("remainder amount is invalid: %s", err)
			}
		}

		return sdk.FormatInvariant(
			types.ModuleName, "valid-remainder-amount",
			msg,
		), broken
	}
}

// BalancedFractionalTotalInvariant checks that the sum of fractional balances
// and the remainder amount is divisible by the conversion factor without any
// leftover amount.
func BalancedFractionalTotalInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		fractionalBalSum := k.GetTotalSumFractionalBalances(ctx)
		remainderAmount := k.GetRemainderAmount(ctx)

		total := fractionalBalSum.Add(remainderAmount)
		fractionalAmount := total.Mod(types.ConversionFactor())

		broken := false
		msg := ""

		if !fractionalAmount.IsZero() {
			broken = true
			msg = fmt.Sprintf(
				"(sum(FractionalBalances) + remainder) %% conversionFactor should be 0 but got %v",
				fractionalAmount,
			)
		}

		return sdk.FormatInvariant(
			types.ModuleName, "balance-remainder-total",
			msg,
		), broken
	}
}

// FractionalDenomNotInBankInvariant checks that the bank does not hold any
// fractional denoms. These assets, e.g. aatom, should only exist in the
// x/precisebank module as this is a decimal extension of uatom that shares
// the same total supply and is effectively the same asset. uatom held by this
// module in x/bank backs all fractional balances in x/precisebank. If aatom
// somehow ends up in x/bank, then it would both break all expectations of this
// module as well as be double-counted in the total supply.
func FractionalDenomNotInBankInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		extBankSupply := k.bk.GetSupply(ctx, types.ExtendedCoinDenom())

		broken := !extBankSupply.IsZero()
		msg := ""

		if broken {
			msg = fmt.Sprintf(
				"x/bank should not hold any %v but has supply of %v",
				types.ExtendedCoinDenom(),
				extBankSupply,
			)
		}

		return sdk.FormatInvariant(
			types.ModuleName, "fractional-denom-not-in-bank",
			msg,
		), broken
	}
}

// TotalSupplyInvariant checks that the total supply of the asset is equal to the sum of the fractional balances and the remainder amount.
func TotalSupplyInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		totalSupply := sdkmath.ZeroInt()

		k.IterateFractionalBalances(ctx, func(addr sdk.AccAddress, amount sdkmath.Int) bool {
			totalSupply = totalSupply.Add(amount)
			return false
		})

		totalSupply = totalSupply.Add(k.GetRemainderAmount(ctx))

		precisebankModuleAddr := k.ak.GetModuleAddress(types.ModuleName)
		k.bk.IterateAllBalances(ctx, func(addr sdk.AccAddress, coin sdk.Coin) bool {
			if addr.Equals(precisebankModuleAddr) {
				return false
			}

			if coin.Denom == types.IntegerCoinDenom() {
				totalSupply = totalSupply.Add(coin.Amount.Mul(types.ConversionFactor()))
			}
			return false
		})

		integerTotalSupply := k.bk.GetSupply(ctx, types.IntegerCoinDenom()).Amount.Mul(types.ConversionFactor())

		broken := !totalSupply.Equal(integerTotalSupply)
		msg := ""

		if broken {
			msg = fmt.Sprintf("total supply %s does not match integer total supply %s", totalSupply, integerTotalSupply)
		}

		return sdk.FormatInvariant(
			types.ModuleName, "total-supply",
			msg,
		), broken
	}
}
