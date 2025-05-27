package keeper

import (
	"github.com/cosmos/evm/x/feemarket/types"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EndBlock calculate base fee and update block gas wanted.
// The EVM end block logic doesn't update the validator set, thus it returns
// an empty slice.
func (k *Keeper) EndBlock(ctx sdk.Context) error {
	baseFee := k.CalculateBaseFee(ctx)

	// return immediately if base fee is nil
	if baseFee.IsNil() {
		return nil
	}

	k.SetBaseFee(ctx, baseFee)

	defer func() {
		floatBaseFee, err := baseFee.Float64()
		if err != nil {
			ctx.Logger().Error("error converting base fee to float64", "error", err.Error())
			return
		}
		// there'll be no panic if fails to convert to float32. Will only loose precision
		telemetry.SetGauge(float32(floatBaseFee), "feemarket", "base_fee")
	}()

	// Store current base fee in event
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypeFeeMarket,
			sdk.NewAttribute(types.AttributeKeyBaseFee, baseFee.String()),
		),
	})

	return nil
}
