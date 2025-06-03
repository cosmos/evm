package keeper

import (
	"errors"
	"fmt"

	"github.com/cosmos/evm/x/feemarket/types"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EndBlock handles both gas tracking and base fee calculation for the next block
func (k *Keeper) EndBlock(ctx sdk.Context) error {
	if ctx.BlockGasMeter() == nil {
		err := errors.New("block gas meter is nil when setting block gas wanted")
		k.Logger(ctx).Error(err.Error())
		return err
	}

	gasWanted := math.NewIntFromUint64(k.GetTransientGasWanted(ctx))
	gasUsed := math.NewIntFromUint64(ctx.BlockGasMeter().GasConsumedToLimit())

	if !gasWanted.IsInt64() {
		err := fmt.Errorf("integer overflow by integer type conversion. Gas wanted > MaxInt64. Gas wanted: %s", gasWanted)
		k.Logger(ctx).Error(err.Error())
		return err
	}

	if !gasUsed.IsInt64() {
		err := fmt.Errorf("integer overflow by integer type conversion. Gas used > MaxInt64. Gas used: %s", gasUsed)
		k.Logger(ctx).Error(err.Error())
		return err
	}

	// to prevent BaseFee manipulation we limit the gasWanted so that
	// gasWanted = max(gasWanted * MinGasMultiplier, gasUsed)
	// this will be keep BaseFee protected from un-penalized manipulation
	// more info here https://github.com/evmos/ethermint/pull/1105#discussion_r888798925
	minGasMultiplier := k.GetParams(ctx).MinGasMultiplier
	limitedGasWanted := math.LegacyNewDec(gasWanted.Int64()).Mul(minGasMultiplier)
	updatedGasWanted := math.LegacyMaxDec(limitedGasWanted, math.LegacyNewDec(gasUsed.Int64())).TruncateInt().Uint64()
	k.SetBlockGasWanted(ctx, updatedGasWanted)

	nextBaseFee := k.CalculateBaseFee(ctx)

	// Set the calculated base fee for use in the next block
	if !nextBaseFee.IsNil() {
		k.SetBaseFee(ctx, nextBaseFee)

		ctx.EventManager().EmitEvents(sdk.Events{
			sdk.NewEvent(
				types.EventTypeFeeMarket,
				sdk.NewAttribute(types.AttributeKeyBaseFee, nextBaseFee.String()),
				sdk.NewAttribute("calculated_at_block", fmt.Sprintf("%d", ctx.BlockHeight())),
			),
		})
	}

	defer func() {
		telemetry.SetGauge(float32(updatedGasWanted), "feemarket", "block_gas")

		if !nextBaseFee.IsNil() {
			floatBaseFee, err := nextBaseFee.Float64()
			if err != nil {
				ctx.Logger().Error("error converting next base fee to float64", "error", err.Error())
				return
			}
			telemetry.SetGauge(float32(floatBaseFee), "feemarket", "next_base_fee")
		}
	}()

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"block_gas",
		sdk.NewAttribute("height", fmt.Sprintf("%d", ctx.BlockHeight())),
		sdk.NewAttribute("amount", fmt.Sprintf("%d", updatedGasWanted)),
	))

	return nil
}
