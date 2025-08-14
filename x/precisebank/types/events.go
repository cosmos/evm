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

func EmitFractionalBalanceChange(
	ctx sdk.Context,
	address sdk.AccAddress,
	beforeAmount sdkmath.Int,
	afterAmount sdkmath.Int,
) {
	delta := afterAmount.Sub(beforeAmount)

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		EventTypeFractionalBalanceChange,
		sdk.NewAttribute(AttributeKeyAddress, address.String()),
		sdk.NewAttribute(AttributeKeyDelta, delta.String()),
	))
}
