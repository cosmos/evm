package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	testconfig "github.com/cosmos/evm/testutil/config"
	"github.com/cosmos/evm/x/precisebank/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestSumExtendedCoin(t *testing.T) {
	chainConfig := testconfig.SixDecimalsChainConfig
	coinInfo := chainConfig.EvmConfig.CoinInfo
	integerDenom := coinInfo.GetDenom()
	extendedDecimals := coinInfo.ExtendedDecimals
	extendedDenom := coinInfo.GetExtendedDenom()

	tests := []struct {
		name string
		amt  sdk.Coins
		want sdk.Coin
	}{
		{
			"empty",
			sdk.NewCoins(),
			sdk.NewCoin(extendedDenom, sdkmath.ZeroInt()),
		},
		{
			"only integer",
			sdk.NewCoins(sdk.NewInt64Coin(integerDenom, 100)),
			sdk.NewCoin(extendedDenom, types.ConversionFactor(extendedDecimals).MulRaw(100)),
		},
		{
			"only extended",
			sdk.NewCoins(sdk.NewInt64Coin(extendedDenom, 100)),
			sdk.NewCoin(extendedDenom, sdkmath.NewInt(100)),
		},
		{
			"integer and extended",
			sdk.NewCoins(
				sdk.NewInt64Coin(integerDenom, 100),
				sdk.NewInt64Coin(extendedDenom, 100),
			),
			sdk.NewCoin(extendedDenom, types.ConversionFactor(extendedDecimals).MulRaw(100).AddRaw(100)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extVal := types.SumExtendedCoin(tt.amt)
			require.Equal(t, tt.want, extVal)
		})
	}
}
