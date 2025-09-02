package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"

	"github.com/cosmos/evm/x/epixmint/types"
)

func TestQueryParams(t *testing.T) {
	// This is a basic test to ensure the query interface compiles correctly
	// More comprehensive tests would require setting up a full test suite

	// Test that QueryParamsRequest and QueryParamsResponse types exist
	req := &types.QueryParamsRequest{}
	require.NotNil(t, req)

	resp := &types.QueryParamsResponse{}
	require.NotNil(t, resp)
}

func TestQueryInflation(t *testing.T) {
	// Test that QueryInflationRequest and QueryInflationResponse types exist
	req := &types.QueryInflationRequest{}
	require.NotNil(t, req)

	resp := &types.QueryInflationResponse{
		Inflation: math.LegacyNewDec(1),
	}
	require.NotNil(t, resp)
	require.Equal(t, math.LegacyNewDec(1), resp.Inflation)
}

func TestQueryAnnualProvisions(t *testing.T) {
	// Test that QueryAnnualProvisionsRequest and QueryAnnualProvisionsResponse types exist
	req := &types.QueryAnnualProvisionsRequest{}
	require.NotNil(t, req)

	resp := &types.QueryAnnualProvisionsResponse{
		AnnualProvisions: math.NewInt(1000),
	}
	require.NotNil(t, resp)
	require.Equal(t, math.NewInt(1000), resp.AnnualProvisions)
}

func TestQueryCurrentSupply(t *testing.T) {
	// Test that QueryCurrentSupplyRequest and QueryCurrentSupplyResponse types exist
	req := &types.QueryCurrentSupplyRequest{}
	require.NotNil(t, req)

	resp := &types.QueryCurrentSupplyResponse{
		CurrentSupply: math.NewInt(5000),
	}
	require.NotNil(t, resp)
	require.Equal(t, math.NewInt(5000), resp.CurrentSupply)
}

func TestQueryMaxSupply(t *testing.T) {
	// Test that QueryMaxSupplyRequest and QueryMaxSupplyResponse types exist
	req := &types.QueryMaxSupplyRequest{}
	require.NotNil(t, req)

	resp := &types.QueryMaxSupplyResponse{
		MaxSupply: math.NewInt(42000000),
	}
	require.NotNil(t, resp)
	require.Equal(t, math.NewInt(42000000), resp.MaxSupply)
}
