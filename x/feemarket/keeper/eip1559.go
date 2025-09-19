package keeper

import (
	"math"

	"github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// CalculateBaseFee calculates the base fee for the current block. This is only calculated once per
// block during BeginBlock. If the NoBaseFee parameter is enabled or below activation height, this function returns nil.
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

	parentGasUsed := k.GetBlockGasWanted(ctx)

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

	factor := evmtypes.GetEVMCoinDecimals().ConversionFactor()
	return types.CalcGasBaseFee(
		parentGasUsed,
		parentGasTargetInt.Uint64(),
		uint64(params.BaseFeeChangeDenominator),
		parentBaseFee,
		sdkmath.LegacyOneDec().QuoInt(factor),
		params.MinGasPrice,
	)
}
