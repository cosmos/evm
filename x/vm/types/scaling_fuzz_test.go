package types_test

import (
	stdmath "math"
	"testing"

	"github.com/stretchr/testify/require"

	testconstants "github.com/cosmos/evm/testutil/constants"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func buildFuzzInputCoins(evmDenom string, amount uint64, includeOtherDenom bool) sdk.Coins {
	input := sdk.NewCoins(sdk.NewCoin(evmDenom, math.NewIntFromUint64(amount)))
	// Guard against uint64 wraparound for amount+1 when amount == MaxUint64.
	if includeOtherDenom && amount < stdmath.MaxUint64 {
		input = input.Add(sdk.NewCoin("other", math.NewIntFromUint64(amount+1))).Sort()
	}

	return input
}

func FuzzConvertCoinsDenomToExtendedDenomWithEvmParams(f *testing.F) {
	f.Add(uint64(0), false)
	f.Add(uint64(1), false)
	f.Add(uint64(1_000_000_000_000_000_000), true)
	f.Add(uint64(42), true)

	coinInfo := testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]
	params := evmtypes.Params{
		EvmDenom: coinInfo.Denom,
		ExtendedDenomOptions: &evmtypes.ExtendedDenomOptions{
			ExtendedDenom: coinInfo.ExtendedDenom,
		},
	}

	f.Fuzz(func(t *testing.T, amount uint64, includeOtherDenom bool) {
		input := buildFuzzInputCoins(coinInfo.Denom, amount, includeOtherDenom)

		converted := evmtypes.ConvertCoinsDenomToExtendedDenomWithEvmParams(input, params)
		convertedAgain := evmtypes.ConvertCoinsDenomToExtendedDenomWithEvmParams(converted, params)

		// Conversion should be idempotent.
		if !convertedAgain.Equal(converted) {
			t.Fatalf("expected idempotent conversion, got %s then %s", converted, convertedAgain)
		}

		// EVM coin amount should be preserved under denom relabeling.
		if converted.AmountOf(coinInfo.ExtendedDenom).String() != math.NewIntFromUint64(amount).String() {
			t.Fatalf("unexpected converted amount: %s", converted.AmountOf(coinInfo.ExtendedDenom))
		}

		// Non-EVM denoms should be preserved.
		if includeOtherDenom && amount < stdmath.MaxUint64 && converted.AmountOf("other").String() != math.NewIntFromUint64(amount+1).String() {
			t.Fatalf("unexpected non-evm denom amount: %s", converted.AmountOf("other"))
		}
	})
}

func TestBuildFuzzInputCoinsMaxUint64Guard(t *testing.T) {
	coinInfo := testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]

	inputAtMax := buildFuzzInputCoins(coinInfo.Denom, stdmath.MaxUint64, true)
	require.Equal(t, "0", inputAtMax.AmountOf("other").String())

	inputBelowMax := buildFuzzInputCoins(coinInfo.Denom, stdmath.MaxUint64-1, true)
	require.Equal(t, math.NewIntFromUint64(stdmath.MaxUint64).String(), inputBelowMax.AmountOf("other").String())
}
