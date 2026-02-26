package types

import (
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// Event types for precisebank operations
	EventTypeFractionalBalanceChange = "fractional_balance_change"

	// Attribute keys
	AttributeKeyAddress = "address"
	AttributeKeyDelta   = "delta"
)

func NewEventFractionalBalanceChange(
	address sdk.AccAddress,
	beforeAmount sdkmath.Int,
	afterAmount sdkmath.Int,
) sdk.Event {
	delta := afterAmount.Sub(beforeAmount)

	return sdk.NewEvent(
		EventTypeFractionalBalanceChange,
		sdk.NewAttribute(AttributeKeyAddress, address.String()),
		sdk.NewAttribute(AttributeKeyDelta, delta.String()),
	)
}

func EmitEventFractionalBalanceChange(
	ctx sdk.Context,
	address sdk.AccAddress,
	beforeAmount sdkmath.Int,
	afterAmount sdkmath.Int,
) {
	ctx.EventManager().EmitEvent(
		NewEventFractionalBalanceChange(address, beforeAmount, afterAmount),
	)
}
