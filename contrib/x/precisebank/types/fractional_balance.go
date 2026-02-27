package types

import (
	"fmt"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ConversionFactor returns a copy of the conversionFactor used to convert the
// fractional balance to integer balances. This is also 1 greater than the max
// valid fractional amount (999_999_999_999):
// 0 < FractionalBalance < conversionFactor
func ConversionFactor() sdkmath.Int {
	return sdkmath.NewIntFromBigInt(evmtypes.GetEVMCoinDecimals().ConversionFactor().BigInt())
}

// IntegerCoinDenom is the denomination for integer coins that are managed by
// x/bank. This is the "true" denomination of the coin, and is also used for
// the reserve to back all fractional coins.
func IntegerCoinDenom() string {
	return evmtypes.GetEVMCoinDenom()
}

// ExtendedCoinDenom is the denomination for the extended IntegerCoinDenom. This
// not only represents the fractional balance, but the total balance of
// integer + fractional balances.
func ExtendedCoinDenom() string {
	return evmtypes.GetEVMCoinExtendedDenom()
}

// IsExtendedDenomSameAsIntegerDenom returns true if the extended denom is the same as the integer denom
// This happens in 18-decimal chains where both denoms are identical
func IsExtendedDenomSameAsIntegerDenom() bool {
	return IntegerCoinDenom() == ExtendedCoinDenom()
}

// FractionalBalance returns a new FractionalBalance with the given address and
// amount.
func NewFractionalBalance(address string, amount sdkmath.Int) FractionalBalance {
	return FractionalBalance{
		Address: address,
		Amount:  amount,
	}
}

// Validate returns an error if the FractionalBalance has an invalid address or
// negative amount.
func (fb FractionalBalance) Validate() error {
	if _, err := sdk.AccAddressFromBech32(fb.Address); err != nil {
		return err
	}

	// Validate the amount with the FractionalAmount wrapper
	return ValidateFractionalAmount(fb.Amount)
}

// ValidateFractionalAmount checks if an sdkmath.Int is a valid fractional
// amount, ensuring it is positive and less than or equal to the maximum
// fractional amount.
func ValidateFractionalAmount(amt sdkmath.Int) error {
	if amt.IsNil() {
		return fmt.Errorf("nil amount")
	}

	if !amt.IsPositive() {
		return fmt.Errorf("non-positive amount %v", amt)
	}

	if amt.GTE(ConversionFactor()) {
		return fmt.Errorf("amount %v exceeds max of %v", amt, ConversionFactor().SubRaw(1))
	}

	return nil
}
