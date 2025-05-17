package keeper

import (
	"errors"
	"fmt"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common/math"
)

// CalculateBaseFee calculates the base fee for the current block. This is calculated at EndBlock
// since applications won't immediately know the base fee of the current block.
// If the NoBaseFee parameter is enabled or below activation height, this function returns nil.
// NOTE: This code is inspired from the go-ethereum EIP1559 implementation and adapted to Cosmos SDK-based
// chains. For the canonical code refer to: https://github.com/ethereum/go-ethereum/blob/master/consensus/misc/eip1559.go
func (k Keeper) CalculateBaseFee(ctx sdk.Context) sdkmath.LegacyDec {
	params := k.GetParams(ctx)

	// Ignore the calculation if not enabled
	if !params.IsBaseFeeEnabled(ctx.BlockHeight()) {
		return sdkmath.LegacyDec{}
	}

	consParams := ctx.ConsensusParams()

	// If the current block is the first EIP-1559 block, return the base fee
	// defined in the parameters (DefaultBaseFee if it hasn't been changed by
	// governance).
	if ctx.BlockHeight() == params.EnableHeight {
		return params.BaseFee
	}

	// get the block gas used and the base fee values for the parent block.
	// NOTE: this is not the parent's base fee but the current block's base fee,
	// as it is retrieved from the transient store, which is committed to the
	// persistent KVStore after EndBlock (ABCI Commit).
	parentBaseFee := params.BaseFee
	if parentBaseFee.IsNil() {
		return sdkmath.LegacyDec{}
	}
	parentGasUsed, err := k.CalculateBlockGasWanted(ctx)
	if err != nil {
		return sdkmath.LegacyDec{}
	}

	gasLimit := sdkmath.NewIntFromUint64(math.MaxUint64)

	// NOTE: a MaxGas equal to -1 means that block gas is unlimited
	if consParams.Block != nil && consParams.Block.MaxGas > -1 {
		gasLimit = sdkmath.NewInt(consParams.Block.MaxGas)
	}

	// CONTRACT: ElasticityMultiplier cannot be 0 as it's checked in the params
	// validation
	parentGasTargetInt := gasLimit.Quo(sdkmath.NewIntFromUint64(uint64(params.ElasticityMultiplier)))
	if !parentGasTargetInt.IsUint64() {
		return sdkmath.LegacyDec{}
	}

	parentGasTarget := parentGasTargetInt.Uint64()
	baseFeeChangeDenominator := sdkmath.NewIntFromUint64(uint64(params.BaseFeeChangeDenominator))

	// If the parent gasUsed is the same as the target, the baseFee remains
	// unchanged.
	if parentGasUsed == parentGasTarget {
		return parentBaseFee
	}

	if parentGasTargetInt.IsZero() {
		return sdkmath.LegacyZeroDec()
	}

	if parentGasUsed > parentGasTarget {
		// If the parent block used more gas than its target, the baseFee should
		// increase.
		gasUsedDelta := sdkmath.NewIntFromUint64(parentGasUsed - parentGasTarget)
		x := parentBaseFee.MulInt(gasUsedDelta)
		y := x.QuoInt(parentGasTargetInt)
		baseFeeDelta := sdkmath.LegacyMaxDec(
			y.QuoInt(baseFeeChangeDenominator),
			sdkmath.LegacyOneDec(),
		)

		return parentBaseFee.Add(baseFeeDelta)
	}

	// Otherwise if the parent block used less gas than its target, the baseFee
	// should decrease.
	gasUsedDelta := sdkmath.NewIntFromUint64(parentGasTarget - parentGasUsed)
	x := parentBaseFee.MulInt(gasUsedDelta)
	y := x.QuoInt(parentGasTargetInt)
	baseFeeDelta := y.QuoInt(baseFeeChangeDenominator)

	// Set global min gas price as lower bound of the base fee, transactions below
	// the min gas price don't even reach the mempool.
	return sdkmath.LegacyMaxDec(parentBaseFee.Sub(baseFeeDelta), params.MinGasPrice)
}

// CalculateBlockGasWanted calculates the block gas wanted based on the current block's gas usage
// and applies the minimum gas multiplier to prevent base fee manipulation.
func (k Keeper) CalculateBlockGasWanted(ctx sdk.Context) (uint64, error) {
	if ctx.BlockGasMeter() == nil {
		err := errors.New("block gas meter is nil when setting block gas wanted")
		k.Logger(ctx).Error(err.Error())
		return 0, err
	}

	gasWanted := sdkmath.NewIntFromUint64(k.GetTransientGasWanted(ctx))
	gasUsed := sdkmath.NewIntFromUint64(ctx.BlockGasMeter().GasConsumedToLimit())

	if !gasWanted.IsInt64() {
		err := fmt.Errorf("integer overflow by integer type conversion. Gas wanted > MaxInt64. Gas wanted: %s", gasWanted)
		k.Logger(ctx).Error(err.Error())
		return 0, err
	}

	if !gasUsed.IsInt64() {
		err := fmt.Errorf("integer overflow by integer type conversion. Gas used > MaxInt64. Gas used: %s", gasUsed)
		k.Logger(ctx).Error(err.Error())
		return 0, err
	}

	// to prevent BaseFee manipulation we limit the gasWanted so that
	// gasWanted = max(gasWanted * MinGasMultiplier, gasUsed)
	// this will be keep BaseFee protected from un-penalized manipulation
	// more info here https://github.com/evmos/ethermint/pull/1105#discussion_r888798925
	minGasMultiplier := k.GetParams(ctx).MinGasMultiplier
	limitedGasWanted := sdkmath.LegacyNewDec(gasWanted.Int64()).Mul(minGasMultiplier)
	updatedGasWanted := sdkmath.LegacyMaxDec(limitedGasWanted, sdkmath.LegacyNewDec(gasUsed.Int64())).TruncateInt().Uint64()
	k.SetBlockGasWanted(ctx, updatedGasWanted)

	defer func() {
		telemetry.SetGauge(float32(updatedGasWanted), "feemarket", "block_gas")
	}()

	ctx.EventManager().EmitEvent(sdk.NewEvent(
		"block_gas",
		sdk.NewAttribute("height", fmt.Sprintf("%d", ctx.BlockHeight())),
		sdk.NewAttribute("amount", fmt.Sprintf("%d", updatedGasWanted)),
	))

	return updatedGasWanted, nil
}
