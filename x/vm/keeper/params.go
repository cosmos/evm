package keeper

import (
	"fmt"
	"slices"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	evmtrace "github.com/cosmos/evm/trace"
	"github.com/cosmos/evm/utils"
	"github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetParams returns the total set of evm parameters.
func (k Keeper) GetParams(ctx sdk.Context) (params types.Params) {
	ctx, span := ctx.StartSpan(tracer, "GetParams")
	defer span.End()
	store := ctx.KVStore(k.storeKey)
	bz := store.Get(types.KeyPrefixParams)
	if bz == nil {
		return params
	}
	k.cdc.MustUnmarshal(bz, &params)
	return params
}

// SetParams sets the EVM params each in their individual key for better get performance
func (k Keeper) SetParams(ctx sdk.Context, params types.Params) (err error) {
	ctx, span := ctx.StartSpan(tracer, "SetParams", trace.WithAttributes(
		attribute.String("evm_denom", params.EvmDenom),
		attribute.Int("active_precompiles_count", len(params.ActiveStaticPrecompiles)),
		attribute.Int("extra_eips_count", len(params.ExtraEIPs)),
	))
	defer func() { evmtrace.EndSpanErr(span, err) }()
	// NOTE: We need to sort the precompiles in order to enable searching with binary search
	// in params.IsActivePrecompile.
	slices.Sort(params.ActiveStaticPrecompiles)

	if err := params.Validate(); err != nil {
		return err
	}

	store := ctx.KVStore(k.storeKey)
	bz, err := k.cdc.Marshal(&params)
	if err != nil {
		return err
	}

	store.Set(types.KeyPrefixParams, bz)
	return nil
}

// EnableStaticPrecompiles appends the addresses of the given Precompiles to the list
// of active static precompiles.
func (k Keeper) EnableStaticPrecompiles(ctx sdk.Context, addresses ...common.Address) (err error) {
	ctx, span := ctx.StartSpan(tracer, "EnableStaticPrecompiles", trace.WithAttributes(
		attribute.Int("addresses_count", len(addresses)),
	))
	defer func() { evmtrace.EndSpanErr(span, err) }()
	params := k.GetParams(ctx)
	activePrecompiles := params.ActiveStaticPrecompiles

	// Append and sort the new precompiles
	updatedPrecompiles, err := appendPrecompiles(activePrecompiles, addresses...)
	if err != nil {
		return err
	}

	params.ActiveStaticPrecompiles = updatedPrecompiles
	return k.SetParams(ctx, params)
}

func appendPrecompiles(existingPrecompiles []string, addresses ...common.Address) ([]string, error) {
	// check for duplicates
	hexAddresses := make([]string, len(addresses))
	for i := range addresses {
		addrHex := addresses[i].Hex()
		if slices.Contains(existingPrecompiles, addrHex) {
			return nil, fmt.Errorf("precompile already registered: %s", addrHex)
		}
		hexAddresses[i] = addrHex
	}

	existingLength := len(existingPrecompiles)
	updatedPrecompiles := make([]string, existingLength+len(hexAddresses))
	copy(updatedPrecompiles, existingPrecompiles)
	copy(updatedPrecompiles[existingLength:], hexAddresses)

	utils.SortSlice(updatedPrecompiles)
	return updatedPrecompiles, nil
}

// EnableEIPs enables the given EIPs in the EVM parameters.
func (k Keeper) EnableEIPs(ctx sdk.Context, eips ...int64) (err error) {
	ctx, span := ctx.StartSpan(tracer, "EnableEIPs", trace.WithAttributes(
		attribute.Int("eips_count", len(eips)),
	))
	defer func() { evmtrace.EndSpanErr(span, err) }()
	evmParams := k.GetParams(ctx)
	evmParams.ExtraEIPs = append(evmParams.ExtraEIPs, eips...)

	sort.Slice(evmParams.ExtraEIPs, func(i, j int) bool {
		return evmParams.ExtraEIPs[i] < evmParams.ExtraEIPs[j]
	})

	return k.SetParams(ctx, evmParams)
}
