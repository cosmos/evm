package keeper

import (
	"context"

	"github.com/cosmos/evm/x/epixmint/types"

	"cosmossdk.io/math"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ types.QueryServer = Keeper{}

// Params returns the total set of epixmint parameters.
func (k Keeper) Params(c context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	params := k.GetParams(c)

	return &types.QueryParamsResponse{Params: params}, nil
}

// Inflation returns the current inflation rate based on the current supply and annual mint amount.
func (k Keeper) Inflation(c context.Context, req *types.QueryInflationRequest) (*types.QueryInflationResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	params := k.GetParams(c)
	currentSupply := k.bankKeeper.GetSupply(c, params.MintDenom)

	// Calculate inflation rate as: current_annual_mint_amount / current_supply
	// If current supply is zero, return zero inflation
	var inflationRate math.LegacyDec
	if currentSupply.Amount.IsZero() {
		inflationRate = math.LegacyZeroDec()
	} else {
		// Get current annual emission rate using dynamic calculation
		currentAnnualMintAmount := calculateCurrentAnnualEmissionRate(c, params)
		// Convert annual mint amount to LegacyDec for division
		annualMintDec := math.LegacyNewDecFromInt(currentAnnualMintAmount)
		currentSupplyDec := math.LegacyNewDecFromInt(currentSupply.Amount)
		inflationRate = annualMintDec.Quo(currentSupplyDec)
	}

	return &types.QueryInflationResponse{
		Inflation: inflationRate,
	}, nil
}

// AnnualProvisions returns the current annual provisions (same as annual mint amount for epixmint).
func (k Keeper) AnnualProvisions(c context.Context, req *types.QueryAnnualProvisionsRequest) (*types.QueryAnnualProvisionsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	params := k.GetParams(c)

	// Get current annual emission rate using dynamic calculation
	currentAnnualProvisions := calculateCurrentAnnualEmissionRate(c, params)

	return &types.QueryAnnualProvisionsResponse{
		AnnualProvisions: currentAnnualProvisions,
	}, nil
}

// CurrentSupply returns the current total supply of the mint denomination.
func (k Keeper) CurrentSupply(c context.Context, req *types.QueryCurrentSupplyRequest) (*types.QueryCurrentSupplyResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	params := k.GetParams(c)
	currentSupply := k.bankKeeper.GetSupply(c, params.MintDenom)

	return &types.QueryCurrentSupplyResponse{
		CurrentSupply: currentSupply.Amount,
	}, nil
}

// MaxSupply returns the maximum supply that can ever be minted.
func (k Keeper) MaxSupply(c context.Context, req *types.QueryMaxSupplyRequest) (*types.QueryMaxSupplyResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	params := k.GetParams(c)

	return &types.QueryMaxSupplyResponse{
		MaxSupply: params.MaxSupply,
	}, nil
}
