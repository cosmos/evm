package common_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/testutil/constants"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var largeAmt, _ = math.NewIntFromString("1000000000000000000000000000000000000000")

func TestNewCoinsResponse(t *testing.T) {
	testCases := []struct {
		amount math.Int
	}{
		{amount: math.NewInt(1)},
		{amount: largeAmt},
	}

	for _, tc := range testCases {
		coin := sdk.NewCoin(constants.ExampleAttoDenom, tc.amount)
		coins := sdk.NewCoins(coin)
		res := common.NewCoinsResponse(coins)
		require.Equal(t, 1, len(res))
		require.Equal(t, tc.amount.BigInt(), res[0].Amount)
	}
}

func TestNewDecCoinsResponse(t *testing.T) {
	testCases := []struct {
		amount math.Int
	}{
		{amount: math.NewInt(1)},
		{amount: largeAmt},
	}

	for _, tc := range testCases {
		coin := sdk.NewDecCoin(constants.ExampleAttoDenom, tc.amount)
		coins := sdk.NewDecCoins(coin)
		res := common.NewDecCoinsResponse(coins)
		require.Equal(t, 1, len(res))
		require.Equal(t, tc.amount.BigInt(), res[0].Amount)
	}
}

// TestNewDecCoinsResponsePrecision checks that Amount scaled by 10^-Precision equals the
// value that NewDecCoinsResponse reports. Amount is the truncated integer amount, so
// Precision must be 0; labelling it 18 makes 1.5 tokens read as 1e-18.
func TestNewDecCoinsResponsePrecision(t *testing.T) {
	testCases := []struct {
		name   string
		amount math.LegacyDec
	}{
		{"whole amount", math.LegacyNewDec(7)},
		{"fractional amount", math.LegacyMustNewDecFromStr("1.5")},
		{"below one", math.LegacyMustNewDecFromStr("0.25")},
		{"large amount", math.LegacyNewDecFromInt(largeAmt)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			coins := sdk.NewDecCoins(sdk.NewDecCoinFromDec(constants.ExampleAttoDenom, tc.amount))
			res := common.NewDecCoinsResponse(coins)
			require.Len(t, res, 1)
			require.Equal(t, tc.amount.TruncateInt().BigInt(), res[0].Amount)

			// value = Amount * 10^-Precision must equal the truncated amount
			value := math.LegacyNewDecFromBigIntWithPrec(res[0].Amount, int64(res[0].Precision))
			require.True(t, value.Equal(math.LegacyNewDecFromInt(tc.amount.TruncateInt())),
				"Amount %s with Precision %d reads as %s, want %s", res[0].Amount, res[0].Precision, value, tc.amount.TruncateInt())
		})
	}
}
