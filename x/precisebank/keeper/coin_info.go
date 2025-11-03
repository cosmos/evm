package keeper

import (
	sdkmath "cosmossdk.io/math"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// SetCoinInfo wires the runtime EVM coin information into the precisebank keeper.
// It must be called before any operations that require denomination or decimal
// conversions.
func (k *Keeper) SetCoinInfo(info evmtypes.EvmCoinInfo) {
	if info.Denom == "" {
		panic("precisebank: coin info must include denom")
	}
	k.coinInfo = info
	k.coinInfoSet = true
}

func (k Keeper) ensureCoinInfoSet() {
	if !k.coinInfoSet {
		panic("precisebank: coin info not initialized")
	}
}

func (k Keeper) evmCoinInfo() evmtypes.EvmCoinInfo {
	k.ensureCoinInfoSet()
	return k.coinInfo
}

func (k Keeper) IntegerDenom() string {
	info := k.evmCoinInfo()
	if info.Denom == "" {
		panic("precisebank: coin info missing denom")
	}
	return info.Denom
}

func (k Keeper) ExtendedDenom() string {
	info := k.evmCoinInfo()
	if info.ExtendedDenom != "" {
		return info.ExtendedDenom
	}
	return info.DenomOrDefault()
}

func (k Keeper) ConversionFactor() sdkmath.Int {
	info := k.evmCoinInfo()
	return info.DecimalsOrDefault().ConversionFactor()
}

// describeCoinInfo is used in panic messages to aid debugging.
