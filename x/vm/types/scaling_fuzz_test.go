package types_test

import (
	"testing"

	testconstants "github.com/cosmos/evm/testutil/constants"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

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
		input := sdk.NewCoins(sdk.NewCoin(coinInfo.Denom, math.NewIntFromUint64(amount)))
		if includeOtherDenom {
			input = input.Add(sdk.NewCoin("other", math.NewIntFromUint64(amount+1))).Sort()
		}

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
		if includeOtherDenom && converted.AmountOf("other").String() != math.NewIntFromUint64(amount+1).String() {
			t.Fatalf("unexpected non-evm denom amount: %s", converted.AmountOf("other"))
		}
	})
}
