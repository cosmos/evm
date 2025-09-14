package keeper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/vm/types"
)

func TestHexToBech32Query(t *testing.T) {
	suite := KeeperTestSuite{}
	suite.SetupTest()

	ctx := context.Background()
	
	testCases := []struct {
		name        string
		request     *types.QueryHexToBech32Request
		expectError bool
		expectedRes string
	}{
		{
			name: "valid hex address conversion",
			request: &types.QueryHexToBech32Request{
				HexAddress: "0x57439ca103e6D052550E5e44586eFE0C80566718",
			},
			expectError: false,
			expectedRes: "epix12apeeggrumg9y4gwtez9smh7pjq9vecc4hc936",
		},
		{
			name: "empty request",
			request: nil,
			expectError: true,
		},
		{
			name: "empty hex address",
			request: &types.QueryHexToBech32Request{
				HexAddress: "",
			},
			expectError: true,
		},
		{
			name: "invalid hex address",
			request: &types.QueryHexToBech32Request{
				HexAddress: "invalid-address",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := suite.app.EvmKeeper.HexToBech32(ctx, tc.request)
			
			if tc.expectError {
				require.Error(t, err)
				require.Nil(t, res)
			} else {
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, tc.expectedRes, res.Bech32Address)
			}
		})
	}
}

func TestBech32ToHexQuery(t *testing.T) {
	suite := KeeperTestSuite{}
	suite.SetupTest()

	ctx := context.Background()
	
	testCases := []struct {
		name        string
		request     *types.QueryBech32ToHexRequest
		expectError bool
		expectedRes string
	}{
		{
			name: "valid bech32 address conversion",
			request: &types.QueryBech32ToHexRequest{
				Bech32Address: "epix12apeeggrumg9y4gwtez9smh7pjq9vecc4hc936",
			},
			expectError: false,
			expectedRes: "0x57439ca103e6D052550E5e44586eFE0C80566718",
		},
		{
			name: "empty request",
			request: nil,
			expectError: true,
		},
		{
			name: "empty bech32 address",
			request: &types.QueryBech32ToHexRequest{
				Bech32Address: "",
			},
			expectError: true,
		},
		{
			name: "invalid bech32 address",
			request: &types.QueryBech32ToHexRequest{
				Bech32Address: "invalid-address",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := suite.app.EvmKeeper.Bech32ToHex(ctx, tc.request)
			
			if tc.expectError {
				require.Error(t, err)
				require.Nil(t, res)
			} else {
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, tc.expectedRes, res.HexAddress)
			}
		})
	}
}

func TestAddressConversionRoundTrip(t *testing.T) {
	suite := KeeperTestSuite{}
	suite.SetupTest()

	ctx := context.Background()
	
	// Test round-trip conversion
	originalHex := "0x57439ca103e6D052550E5e44586eFE0C80566718"
	
	// Convert hex to bech32
	hexToBech32Req := &types.QueryHexToBech32Request{
		HexAddress: originalHex,
	}
	bech32Res, err := suite.app.EvmKeeper.HexToBech32(ctx, hexToBech32Req)
	require.NoError(t, err)
	require.NotNil(t, bech32Res)
	
	// Convert bech32 back to hex
	bech32ToHexReq := &types.QueryBech32ToHexRequest{
		Bech32Address: bech32Res.Bech32Address,
	}
	hexRes, err := suite.app.EvmKeeper.Bech32ToHex(ctx, bech32ToHexReq)
	require.NoError(t, err)
	require.NotNil(t, hexRes)
	
	// Verify round-trip conversion
	require.Equal(t, originalHex, hexRes.HexAddress)
}
