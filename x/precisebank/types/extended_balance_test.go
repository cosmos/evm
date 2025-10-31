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
			sdk.NewCoin(types.ExtendedCoinDenom(), sdkmath.ZeroInt()),
		},
		{
			"only integer",
			sdk.NewCoins(sdk.NewInt64Coin(types.IntegerCoinDenom(), 100)),
			sdk.NewCoin(types.ExtendedCoinDenom(), types.ConversionFactor().MulRaw(100)),
		},
		{
			"only extended",
			sdk.NewCoins(sdk.NewInt64Coin(types.ExtendedCoinDenom(), 100)),
			sdk.NewCoin(types.ExtendedCoinDenom(), sdkmath.NewInt(100)),
		},
		{
			"integer and extended",
			sdk.NewCoins(
				sdk.NewInt64Coin(types.IntegerCoinDenom(), 100),
				sdk.NewInt64Coin(types.ExtendedCoinDenom(), 100),
			),
			sdk.NewCoin(types.ExtendedCoinDenom(), types.ConversionFactor().MulRaw(100).AddRaw(100)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extVal := types.SumExtendedCoin(tt.amt)
			require.Equal(t, tt.want, extVal)
		})
	}
}
