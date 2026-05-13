package types_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	testconstants "github.com/cosmos/evm/testutil/constants"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestConvertEvmCoinFrom18Decimals(t *testing.T) {
	eighteenDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]

	eighteenDecimalsBaseCoinZero := sdk.Coin{Denom: eighteenDecimalsCoinInfo.Denom, Amount: math.NewInt(0)}

	testCases := []struct {
		name        string
		evmCoinInfo evmtypes.EvmCoinInfo
		coin        sdk.Coin
		expCoin     sdk.Coin
		expErr      bool
	}{
		{
			name:        "pass - zero amount 18 decimals",
			evmCoinInfo: eighteenDecimalsCoinInfo,
			coin:        eighteenDecimalsBaseCoinZero,
			expErr:      false,
			expCoin:     eighteenDecimalsBaseCoinZero,
		},
		{
			name:        "pass - no conversion with 18 decimals",
			evmCoinInfo: eighteenDecimalsCoinInfo,
			coin:        sdk.Coin{Denom: eighteenDecimalsCoinInfo.Denom, Amount: math.NewInt(10)},
			expErr:      false,
			expCoin:     sdk.Coin{Denom: eighteenDecimalsCoinInfo.Denom, Amount: math.NewInt(10)},
		},
		{
			name:        "fail - not evm denom",
			evmCoinInfo: eighteenDecimalsCoinInfo,
			coin:        sdk.Coin{Denom: "atom", Amount: math.NewInt(1)},
			expErr:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configurator := evmtypes.NewEVMConfigurator()
			configurator.ResetTestConfig()
			require.NoError(t, configurator.WithEVMCoinInfo(tc.evmCoinInfo).Configure())

			coinConverted, err := evmtypes.ConvertEvmCoinDenomToExtendedDenom(tc.coin)

			if !tc.expErr {
				require.NoError(t, err)
				require.Equal(t, tc.expCoin, coinConverted, "expected a different coin")
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestConvertCoinsFrom18Decimals(t *testing.T) {
	eighteenDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]

	nonBaseCoin := sdk.Coin{Denom: "btc", Amount: math.NewInt(10)}
	eighteenDecimalsBaseCoin := sdk.Coin{Denom: eighteenDecimalsCoinInfo.Denom, Amount: math.NewInt(10)}

	testCases := []struct {
		name        string
		evmCoinInfo evmtypes.EvmCoinInfo
		coins       sdk.Coins
		expCoins    sdk.Coins
	}{
		{
			name:        "pass - no evm denom",
			evmCoinInfo: eighteenDecimalsCoinInfo,
			coins:       sdk.Coins{nonBaseCoin},
			expCoins:    sdk.Coins{nonBaseCoin},
		},
		{
			name:        "pass - only base denom 18 decimals",
			evmCoinInfo: eighteenDecimalsCoinInfo,
			coins:       sdk.Coins{eighteenDecimalsBaseCoin},
			expCoins:    sdk.Coins{eighteenDecimalsBaseCoin},
		},
		{
			name:        "pass - multiple coins and base denom 18 decimals",
			evmCoinInfo: eighteenDecimalsCoinInfo,
			coins:       sdk.Coins{nonBaseCoin, eighteenDecimalsBaseCoin}.Sort(),
			expCoins:    sdk.Coins{nonBaseCoin, eighteenDecimalsBaseCoin}.Sort(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configurator := evmtypes.NewEVMConfigurator()
			configurator.ResetTestConfig()
			require.NoError(t, configurator.WithEVMCoinInfo(tc.evmCoinInfo).Configure())

			coinConverted := evmtypes.ConvertCoinsDenomToExtendedDenom(tc.coins)
			require.Equal(t, tc.expCoins, coinConverted, "expected a different coin")
		})
	}
}

func TestConvertAmountTo18DecimalsLegacy(t *testing.T) {
	testCases := []struct {
		name string
		amt  *uint256.Int
	}{
		{
			name: "smallest amount",
			amt:  uint256.NewInt(1),
		},
		{
			name: "almost 1: 0.99999...",
			amt:  uint256.NewInt(999999999999),
		},
		{
			name: "half of the minimum uint",
			amt:  uint256.NewInt(5e11),
		},
		{
			name: "one int",
			amt:  uint256.NewInt(1e12),
		},
		{
			name: "one 'ether'",
			amt:  uint256.NewInt(1e18),
		},
	}

	coinInfo := testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d dec - %s", coinInfo.Decimals, tc.name), func(t *testing.T) {
			configurator := evmtypes.NewEVMConfigurator()
			configurator.ResetTestConfig()
			require.NoError(t, configurator.WithEVMCoinInfo(coinInfo).Configure())
			res := evmtypes.ConvertBigIntFrom18DecimalsToLegacyDec(tc.amt.ToBig())
			require.Equal(t, math.LegacyNewDecFromBigInt(tc.amt.ToBig()), res)
		})
	}
}

func TestConvertAmountTo18DecimalsBigInt(t *testing.T) {
	testCases := []struct {
		name     string
		amt      *big.Int
		expected *big.Int
	}{
		{
			name:     "one int",
			amt:      big.NewInt(1),
			expected: big.NewInt(1),
		},
		{
			name:     "one 'ether'",
			amt:      big.NewInt(1e6),
			expected: big.NewInt(1e6),
		},
	}

	coinInfo := testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d dec - %s", coinInfo.Decimals, tc.name), func(t *testing.T) {
			configurator := evmtypes.NewEVMConfigurator()
			configurator.ResetTestConfig()
			require.NoError(t, configurator.WithEVMCoinInfo(coinInfo).Configure())
			res := evmtypes.ConvertAmountTo18DecimalsBigInt(tc.amt)
			require.Equal(t, tc.expected, res)
		})
	}
}

func TestConvertCoinsDenomToExtendedDenomWithEvmParams(t *testing.T) {
	eighteenDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]
	eighteenDecimalsParams := evmtypes.Params{
		EvmDenom: eighteenDecimalsCoinInfo.Denom,
		ExtendedDenomOptions: &evmtypes.ExtendedDenomOptions{
			ExtendedDenom: eighteenDecimalsCoinInfo.ExtendedDenom,
		},
	}
	nonBaseCoin := sdk.Coin{Denom: "btc", Amount: math.NewInt(100)}
	eighteenDecimalsBaseCoin := sdk.Coin{Denom: eighteenDecimalsCoinInfo.Denom, Amount: math.NewInt(1000000000000000000)}

	tcs := []struct {
		name     string
		coins    sdk.Coins
		params   evmtypes.Params
		expected sdk.Coins
	}{
		{
			name:     "empty coins",
			coins:    sdk.Coins{},
			params:   eighteenDecimalsParams,
			expected: sdk.Coins{},
		},
		{
			name:  "single coin - 18 decimals (no conversion)",
			coins: sdk.NewCoins(eighteenDecimalsBaseCoin),
			params: evmtypes.Params{
				EvmDenom: eighteenDecimalsCoinInfo.Denom,
				ExtendedDenomOptions: &evmtypes.ExtendedDenomOptions{
					ExtendedDenom: eighteenDecimalsCoinInfo.ExtendedDenom,
				},
			},
			expected: sdk.NewCoins(sdk.Coin{Denom: eighteenDecimalsCoinInfo.ExtendedDenom, Amount: math.NewInt(1000000000000000000)}),
		},
		{
			name:     "single coin - different denom (no conversion)",
			coins:    sdk.NewCoins(nonBaseCoin),
			params:   eighteenDecimalsParams,
			expected: sdk.NewCoins(nonBaseCoin),
		},
		{
			name: "multiple coins - mixed denominations",
			coins: sdk.NewCoins(
				eighteenDecimalsBaseCoin,
				nonBaseCoin,
			).Sort(),
			params: eighteenDecimalsParams,
			expected: sdk.NewCoins(
				nonBaseCoin,
				sdk.Coin{Denom: eighteenDecimalsCoinInfo.ExtendedDenom, Amount: math.NewInt(1000000000000000000)},
			).Sort(),
		},
		{
			name:     "zero amount coin",
			coins:    sdk.NewCoins(sdk.Coin{Denom: eighteenDecimalsCoinInfo.Denom, Amount: math.NewInt(0)}),
			params:   eighteenDecimalsParams,
			expected: sdk.NewCoins(sdk.Coin{Denom: eighteenDecimalsCoinInfo.ExtendedDenom, Amount: math.NewInt(0)}),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			result := evmtypes.ConvertCoinsDenomToExtendedDenomWithEvmParams(tc.coins, tc.params)
			require.Equal(t, tc.expected, result)
		})
	}
}
