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
	sixDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.SixDecimalsChainID]

	eighteenDecimalsBaseCoinZero := sdk.Coin{Denom: eighteenDecimalsCoinInfo.Denom, Amount: math.NewInt(0)}
	sixDecimalsBaseCoinZero := sdk.Coin{Denom: sixDecimalsCoinInfo.Denom, Amount: math.NewInt(0)}

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
			name:        "pass - zero amount 6 decimals",
			evmCoinInfo: sixDecimalsCoinInfo,
			coin:        sixDecimalsBaseCoinZero,
			expErr:      false,
			expCoin:     sdk.Coin{Denom: sixDecimalsCoinInfo.ExtendedDenom, Amount: math.NewInt(0)},
		},
		{
			name:        "pass - no conversion with 18 decimals",
			evmCoinInfo: eighteenDecimalsCoinInfo,
			coin:        sdk.Coin{Denom: eighteenDecimalsCoinInfo.Denom, Amount: math.NewInt(10)},
			expErr:      false,
			expCoin:     sdk.Coin{Denom: eighteenDecimalsCoinInfo.Denom, Amount: math.NewInt(10)},
		},
		{
			name:        "pass - conversion with 6 decimals",
			evmCoinInfo: sixDecimalsCoinInfo,
			coin:        sdk.Coin{Denom: sixDecimalsCoinInfo.Denom, Amount: math.NewInt(1e12)},
			expErr:      false,
			expCoin:     sdk.Coin{Denom: sixDecimalsCoinInfo.ExtendedDenom, Amount: math.NewInt(1e12)},
		},
		{
			name:        "pass - conversion with amount less than conversion factor",
			evmCoinInfo: sixDecimalsCoinInfo,
			coin:        sdk.Coin{Denom: sixDecimalsCoinInfo.Denom, Amount: math.NewInt(1e11)},
			expErr:      false,
			expCoin:     sdk.Coin{Denom: sixDecimalsCoinInfo.ExtendedDenom, Amount: math.NewInt(1e11)},
		},
		{
			name:        "fail - not evm denom",
			evmCoinInfo: sixDecimalsCoinInfo,
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
	sixDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.SixDecimalsChainID]

	nonBaseCoin := sdk.Coin{Denom: "btc", Amount: math.NewInt(10)}
	eighteenDecimalsBaseCoin := sdk.Coin{Denom: eighteenDecimalsCoinInfo.Denom, Amount: math.NewInt(10)}
	sixDecimalsBaseCoin := sdk.Coin{Denom: sixDecimalsCoinInfo.Denom, Amount: math.NewInt(10)}

	testCases := []struct {
		name        string
		evmCoinInfo evmtypes.EvmCoinInfo
		coins       sdk.Coins
		expCoins    sdk.Coins
	}{
		{
			name:        "pass - no evm denom",
			evmCoinInfo: sixDecimalsCoinInfo,
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
			name:        "pass - only base denom 6 decimals",
			evmCoinInfo: sixDecimalsCoinInfo,
			coins:       sdk.Coins{sixDecimalsBaseCoin},
			expCoins:    sdk.Coins{sdk.Coin{Denom: sixDecimalsCoinInfo.ExtendedDenom, Amount: math.NewInt(10)}},
		},
		{
			name:        "pass - multiple coins and base denom 18 decimals",
			evmCoinInfo: eighteenDecimalsCoinInfo,
			coins:       sdk.Coins{nonBaseCoin, eighteenDecimalsBaseCoin}.Sort(),
			expCoins:    sdk.Coins{nonBaseCoin, eighteenDecimalsBaseCoin}.Sort(),
		},
		{
			name:        "pass - multiple coins and base denom 6 decimals",
			evmCoinInfo: sixDecimalsCoinInfo,
			coins:       sdk.Coins{nonBaseCoin, sixDecimalsBaseCoin}.Sort(),
			expCoins:    sdk.Coins{nonBaseCoin, sdk.Coin{Denom: sixDecimalsCoinInfo.ExtendedDenom, Amount: math.NewInt(10)}}.Sort(),
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
		name    string
		amt     *uint256.Int
		exp6dec math.LegacyDec
	}{
		{
			name:    "smallest amount",
			amt:     uint256.NewInt(1),
			exp6dec: math.LegacyMustNewDecFromStr("0.000000000001"),
		},
		{
			name:    "almost 1: 0.99999...",
			amt:     uint256.NewInt(999999999999),
			exp6dec: math.LegacyMustNewDecFromStr("0.999999999999"),
		},
		{
			name:    "half of the minimum uint",
			amt:     uint256.NewInt(5e11),
			exp6dec: math.LegacyMustNewDecFromStr("0.5"),
		},
		{
			name:    "one int",
			amt:     uint256.NewInt(1e12),
			exp6dec: math.LegacyOneDec(),
		},
		{
			name:    "one 'ether'",
			amt:     uint256.NewInt(1e18),
			exp6dec: math.LegacyNewDec(1e6),
		},
	}

	for _, coinInfo := range []evmtypes.EvmCoinInfo{
		testconstants.ExampleChainCoinInfo[testconstants.SixDecimalsChainID],
		testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID],
	} {
		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%d dec - %s", coinInfo.Decimals, tc.name), func(t *testing.T) {
				configurator := evmtypes.NewEVMConfigurator()
				configurator.ResetTestConfig()
				require.NoError(t, configurator.WithEVMCoinInfo(coinInfo).Configure())
				res := evmtypes.ConvertBigIntFrom18DecimalsToLegacyDec(tc.amt.ToBig())
				exp := math.LegacyNewDecFromBigInt(tc.amt.ToBig())
				if coinInfo.Decimals == evmtypes.SixDecimals {
					exp = tc.exp6dec
				}
				require.Equal(t, exp, res)
			})
		}
	}
}

func TestConvertAmountTo18DecimalsBigInt(t *testing.T) {
	testCases := []struct {
		name    string
		amt     *big.Int
		exp6dec *big.Int
	}{
		{
			name:    "one int",
			amt:     big.NewInt(1),
			exp6dec: big.NewInt(1e12),
		},
		{
			name:    "one 'ether'",
			amt:     big.NewInt(1e6),
			exp6dec: big.NewInt(1e18),
		},
	}

	for _, coinInfo := range []evmtypes.EvmCoinInfo{
		testconstants.ExampleChainCoinInfo[testconstants.SixDecimalsChainID],
		testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID],
	} {
		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%d dec - %s", coinInfo.Decimals, tc.name), func(t *testing.T) {
				configurator := evmtypes.NewEVMConfigurator()
				configurator.ResetTestConfig()
				require.NoError(t, configurator.WithEVMCoinInfo(coinInfo).Configure())
				res := evmtypes.ConvertAmountTo18DecimalsBigInt(tc.amt)
				exp := tc.amt
				if coinInfo.Decimals == evmtypes.SixDecimals {
					exp = tc.exp6dec
				}
				require.Equal(t, exp, res)
			})
		}
	}
}
