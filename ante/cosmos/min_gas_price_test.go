package cosmos_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/ante/cosmos"
	"github.com/stretchr/testify/require"
)

func TestIsValidFeeCoins(t *testing.T) {
	tests := []struct {
		name           string
		feeCoins       sdk.Coins
		allowedDenoms  []string
		expectedResult bool
	}{
		{
			name:           "empty fee coins should be valid",
			feeCoins:       sdk.Coins{},
			allowedDenoms:  []string{"uatom"},
			expectedResult: true,
		},
		{
			name:           "nil fee coins should be valid",
			feeCoins:       nil,
			allowedDenoms:  []string{"uatom"},
			expectedResult: true,
		},
		{
			name:           "single allowed denom should be valid",
			feeCoins:       sdk.NewCoins(sdk.NewCoin("uatom", sdkmath.NewInt(1000))),
			allowedDenoms:  []string{"uatom"},
			expectedResult: true,
		},
		{
			name:           "single allowed denom from multiple allowed should be valid",
			feeCoins:       sdk.NewCoins(sdk.NewCoin("uatom", sdkmath.NewInt(1000))),
			allowedDenoms:  []string{"uatom", "stake", "wei"},
			expectedResult: true,
		},
		{
			name:           "single disallowed denom should be invalid",
			feeCoins:       sdk.NewCoins(sdk.NewCoin("forbidden", sdkmath.NewInt(1000))),
			allowedDenoms:  []string{"uatom"},
			expectedResult: false,
		},
		{
			name:           "single disallowed denom with empty allowed list should be invalid",
			feeCoins:       sdk.NewCoins(sdk.NewCoin("uatom", sdkmath.NewInt(1000))),
			allowedDenoms:  []string{},
			expectedResult: false,
		},
		{
			name:           "multiple fee coins should be invalid",
			feeCoins:       sdk.NewCoins(sdk.NewCoin("uatom", sdkmath.NewInt(1000)), sdk.NewCoin("stake", sdkmath.NewInt(500))),
			allowedDenoms:  []string{"uatom", "stake"},
			expectedResult: false,
		},
		{
			name:           "multiple fee coins with one allowed should be invalid",
			feeCoins:       sdk.NewCoins(sdk.NewCoin("uatom", sdkmath.NewInt(1000)), sdk.NewCoin("forbidden", sdkmath.NewInt(500))),
			allowedDenoms:  []string{"uatom"},
			expectedResult: false,
		},
		{
			name:           "empty allowed denoms with empty fee coins should be valid",
			feeCoins:       sdk.Coins{},
			allowedDenoms:  []string{},
			expectedResult: true,
		},
		{
			name:           "case sensitive denom matching",
			feeCoins:       sdk.NewCoins(sdk.NewCoin("UATOM", sdkmath.NewInt(1000))),
			allowedDenoms:  []string{"uatom"},
			expectedResult: false,
		},
		{
			name:           "zero amount allowed denom should be valid",
			feeCoins:       sdk.NewCoins(sdk.NewCoin("uatom", sdkmath.NewInt(0))),
			allowedDenoms:  []string{"uatom"},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosmos.IsValidFeeCoins(tt.feeCoins, tt.allowedDenoms)
			require.Equal(t, tt.expectedResult, result, "IsValidFeeCoins returned unexpected result")
		})
	}
}
