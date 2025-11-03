package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/precisebank/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestSumExtendedCoin(t *testing.T) {
	tests := []struct {
		name string
		amt  sdk.Coins
		want sdk.Coin
	}{
		{
			"empty",
			sdk.NewCoins(),
			sdk.NewCoin(testExtendedDenom, sdkmath.ZeroInt()),
		},
		{
			"only integer",
			sdk.NewCoins(sdk.NewInt64Coin(testIntegerDenom, 100)),
			sdk.NewCoin(testExtendedDenom, testConversionFactor.MulRaw(100)),
		},
		{
			"only extended",
			sdk.NewCoins(sdk.NewInt64Coin(testExtendedDenom, 100)),
			sdk.NewCoin(testExtendedDenom, sdkmath.NewInt(100)),
		},
		{
			"integer and extended",
			sdk.NewCoins(
				sdk.NewInt64Coin(testIntegerDenom, 100),
				sdk.NewInt64Coin(testExtendedDenom, 100),
			),
			sdk.NewCoin(testExtendedDenom, testConversionFactor.MulRaw(100).AddRaw(100)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extVal := types.SumExtendedCoin(tt.amt, testCoinInfo)
			require.Equal(t, tt.want, extVal)
		})
	}
}
