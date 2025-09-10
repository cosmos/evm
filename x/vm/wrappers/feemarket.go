package wrappers

import (
	"math/big"

	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// FeeMarketWrapper is a wrapper around the feemarket keeper
// that is used to manage an evm denom with 6 or 18 decimals.
// The wrapper makes the corresponding conversions to achieve:
//   - With the EVM, the wrapper works always with 18 decimals.
//   - With the feemarket module, the wrapper works always
//     with the bank module decimals (either 6 or 18).
type FeeMarketWrapper struct {
	types.FeeMarketKeeper
	evmCfg *types.EvmConfig
}

// NewFeeMarketWrapper creates a new feemarket Keeper wrapper instance.
// The BankWrapper is used to manage an evm denom with 6 or 18 decimals.
func NewFeeMarketWrapper(
	fk types.FeeMarketKeeper,
	evmCfg *types.EvmConfig,
) *FeeMarketWrapper {
	return &FeeMarketWrapper{
		fk,
		evmCfg,
	}
}

// GetBaseFee returns the base fee converted to 18 decimals.
func (w FeeMarketWrapper) GetBaseFee(ctx sdk.Context) *big.Int {
	baseFee := w.FeeMarketKeeper.GetBaseFee(ctx)
	if baseFee.IsNil() {
		return nil
	}
	return types.ConvertAmountTo18DecimalsLegacy(baseFee, w.evmCfg.CoinInfo.Decimals).TruncateInt().BigInt()
}

// CalculateBaseFee returns the calculated base fee converted to 18 decimals.
func (w FeeMarketWrapper) CalculateBaseFee(ctx sdk.Context) *big.Int {
	baseFee := w.FeeMarketKeeper.CalculateBaseFee(ctx)
	if baseFee.IsNil() {
		return nil
	}
	baseDecimals := w.evmCfg.CoinInfo.Decimals
	return types.ConvertAmountTo18DecimalsLegacy(baseFee, baseDecimals).TruncateInt().BigInt()
}

// GetParams returns the params with associated fees values converted to 18 decimals.
func (w FeeMarketWrapper) GetParams(ctx sdk.Context) feemarkettypes.Params {
	baseDecimals := w.evmCfg.CoinInfo.Decimals

	params := w.FeeMarketKeeper.GetParams(ctx)
	if !params.BaseFee.IsNil() {
		params.BaseFee = types.ConvertAmountTo18DecimalsLegacy(params.BaseFee, baseDecimals)
	}
	params.MinGasPrice = types.ConvertAmountTo18DecimalsLegacy(params.MinGasPrice, baseDecimals)
	return params
}
