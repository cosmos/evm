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
func ConversionFactor(decimals evmtypes.Decimals) sdkmath.Int {
	return sdkmath.NewIntFromBigInt(decimals.ConversionFactor().BigInt())
}

// IsDenomSameAsExtendedDenom returns true if the denom is the same as the extended denom
// This happens in 18-decimal chains where both denoms are identical
func IsDenomSameAsExtendedDenom(coinInfo evmtypes.EvmCoinInfo) bool {
	return coinInfo.GetDenom() == coinInfo.GetExtendedDenom()
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
func (fb FractionalBalance) Validate(decimals evmtypes.Decimals) error {
	if _, err := sdk.AccAddressFromBech32(fb.Address); err != nil {
		return err
	}

	// Validate the amount with the FractionalAmount wrapper
	return ValidateFractionalAmount(fb.Amount, decimals)
}

// ValidateFractionalAmount checks if an sdkmath.Int is a valid fractional
// amount, ensuring it is positive and less than or equal to the maximum
// fractional amount.
func ValidateFractionalAmount(amt sdkmath.Int, decimals evmtypes.Decimals) error {
	if amt.IsNil() {
		return fmt.Errorf("nil amount")
	}

	if !amt.IsPositive() {
		return fmt.Errorf("non-positive amount %v", amt)
	}

	convFactor := ConversionFactor(decimals)
	if amt.GTE(convFactor) {
		return fmt.Errorf("amount %v exceeds max of %v", amt, convFactor.SubRaw(1))
	}

	return nil
}
