package types

import (
	"fmt"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

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
func (fb FractionalBalance) Validate(conversionFactor sdkmath.Int) error {
	if _, err := sdk.AccAddressFromBech32(fb.Address); err != nil {
		return err
	}

	// Validate the amount with the FractionalAmount wrapper
	return ValidateFractionalAmount(fb.Amount, conversionFactor)
}

// ValidateFractionalAmount checks if an sdkmath.Int is a valid fractional
// amount, ensuring it is positive and less than or equal to the maximum
// fractional amount.
func ValidateFractionalAmount(amt, conversionFactor sdkmath.Int) error {
	if amt.IsNil() {
		return fmt.Errorf("nil amount")
	}

	if !amt.IsPositive() {
		return fmt.Errorf("non-positive amount %v", amt)
	}

	if amt.GTE(conversionFactor) {
		return fmt.Errorf("amount %v exceeds max of %v", amt, conversionFactor.SubRaw(1))
	}

	return nil
}
