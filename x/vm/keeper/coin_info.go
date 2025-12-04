package keeper

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// LoadEvmCoinInfo load EvmCoinInfo from bank denom metadata
func (k Keeper) LoadEvmCoinInfo(ctx sdk.Context) (_ types.EvmCoinInfo, err error) {
	params := k.GetParams(ctx)
	ctx, span := ctx.StartSpan(tracer, "LoadEvmCoinInfo", trace.WithAttributes(
		attribute.String("evm_denom", params.EvmDenom),
	))
	defer func() { span.RecordError(err) }()
	defer span.End()
	var decimals types.Decimals
	evmDenomMetadata, found := k.bankWrapper.GetDenomMetaData(ctx, params.EvmDenom)
	if !found {
		return types.EvmCoinInfo{}, fmt.Errorf("denom metadata %s could not be found", params.EvmDenom)
	}

	for _, denomUnit := range evmDenomMetadata.DenomUnits {
		if denomUnit.Denom == evmDenomMetadata.Display {
			decimals = types.Decimals(denomUnit.Exponent)
		}
	}

	var extendedDenom string
	if decimals == 18 {
		extendedDenom = params.EvmDenom
	} else {
		if params.ExtendedDenomOptions == nil {
			return types.EvmCoinInfo{}, fmt.Errorf("extended denom options cannot be nil for non-18-decimal chains")
		}
		extendedDenom = params.ExtendedDenomOptions.ExtendedDenom
	}

	return types.EvmCoinInfo{
		Denom:         params.EvmDenom,
		ExtendedDenom: extendedDenom,
		DisplayDenom:  evmDenomMetadata.Display,
		Decimals:      decimals.Uint32(),
	}, nil
}

// InitEvmCoinInfo load EvmCoinInfo from bank denom metadata and store it in the module
func (k Keeper) InitEvmCoinInfo(ctx sdk.Context) (err error) {
	ctx, span := ctx.StartSpan(tracer, "InitEvmCoinInfo")
	defer func() { span.RecordError(err) }()
	defer span.End()
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
	defer func() { span.RecordError(err) }()
	defer span.End()
	store := ctx.KVStore(k.storeKey)
	bz, err := k.cdc.Marshal(&coinInfo)
	if err != nil {
		return err
	}

	store.Set(types.KeyPrefixEvmCoinInfo, bz)
	return nil
}
