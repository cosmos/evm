package keeper

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	evmtrace "github.com/cosmos/evm/trace"
	"github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// LoadEvmCoinInfo loads EvmCoinInfo from bank denom metadata.
// For 18-decimal chains: evm_denom is the base (exp=0), extended_denom = evm_denom.
// For non-18-decimal chains: evm_denom has exp>0, extended_denom required.
func (k Keeper) LoadEvmCoinInfo(ctx sdk.Context) (_ types.EvmCoinInfo, err error) {
	params := k.GetParams(ctx)
	ctx, span := ctx.StartSpan(tracer, "LoadEvmCoinInfo", trace.WithAttributes(
		attribute.String("evm_denom", params.EvmDenom),
	))
	defer func() { evmtrace.EndSpanErr(span, err) }()

	// try evm_denom first, then extended_denom
	metadata, found := k.bankWrapper.GetDenomMetaData(ctx, params.EvmDenom)
	if !found && params.ExtendedDenomOptions != nil {
		metadata, found = k.bankWrapper.GetDenomMetaData(ctx, params.ExtendedDenomOptions.ExtendedDenom)
	}
	if !found {
		return types.EvmCoinInfo{}, fmt.Errorf("denom metadata not found for evm_denom %s", params.EvmDenom)
	}

	// Extract exponents from denom_units
	exponents := make(map[string]uint32)
	for _, unit := range metadata.DenomUnits {
		exponents[unit.Denom] = unit.Exponent
	}

	evmDenomExp, ok := exponents[params.EvmDenom]
	if !ok {
		return types.EvmCoinInfo{}, fmt.Errorf("evm_denom %s not found in denom_units", params.EvmDenom)
	}

	extendedDenom := params.EvmDenom
	decimals := evmDenomExp
	if evmDenomExp == 0 {
		// evm_denom is the base, 18-decimal chain
		decimals = 18
	} else if params.ExtendedDenomOptions != nil {
		// Non-18-decimal chain, use extended_denom
		extendedDenom = params.ExtendedDenomOptions.ExtendedDenom
	} else {
		return types.EvmCoinInfo{}, fmt.Errorf("extended_denom_options required when evm_denom exp > 0")
	}

	return types.EvmCoinInfo{
		Denom:         params.EvmDenom,
		ExtendedDenom: extendedDenom,
		DisplayDenom:  metadata.Display,
		Decimals:      decimals,
	}, nil
}

// InitEvmCoinInfo load EvmCoinInfo from bank denom metadata and store it in the module
func (k Keeper) InitEvmCoinInfo(ctx sdk.Context) (err error) {
	ctx, span := ctx.StartSpan(tracer, "InitEvmCoinInfo")
	defer func() { evmtrace.EndSpanErr(span, err) }()
	coinInfo, err := k.LoadEvmCoinInfo(ctx)
	if err != nil {
		return err
	}
	return k.SetEvmCoinInfo(ctx, coinInfo)
}

// GetEvmCoinInfo returns the EVM Coin Info stored in the module
func (k Keeper) GetEvmCoinInfo(ctx sdk.Context) (coinInfo types.EvmCoinInfo) {
	ctx, span := ctx.StartSpan(tracer, "GetEvmCoinInfo")
	defer span.End()
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.KeyPrefixEvmCoinInfo)
	if bz == nil {
		return k.defaultEvmCoinInfo
	}
	k.cdc.MustUnmarshal(bz, &coinInfo)
	return
}

// SetEvmCoinInfo sets the EVM Coin Info stored in the module
func (k Keeper) SetEvmCoinInfo(ctx sdk.Context, coinInfo types.EvmCoinInfo) (err error) {
	ctx, span := ctx.StartSpan(tracer, "SetEvmCoinInfo", trace.WithAttributes(
		attribute.String("denom", coinInfo.Denom),
		attribute.Int64("decimals", int64(coinInfo.Decimals)),
	))
	defer func() { evmtrace.EndSpanErr(span, err) }()
	store := ctx.KVStore(k.storeKey)
	bz, err := k.cdc.Marshal(&coinInfo)
	if err != nil {
		return err
	}

	store.Set(types.KeyPrefixEvmCoinInfo, bz)
	return nil
}
