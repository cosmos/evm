package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvmCoinInfoValidate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		coinInfo    EvmCoinInfo
		expPass     bool
		errContains string
	}{
		{
			name: "valid 18 decimals config",
			coinInfo: EvmCoinInfo{
				Denom:         "atest",
				ExtendedDenom: "atest",
				DisplayDenom:  "test",
				Decimals:      EighteenDecimals,
			},
			expPass: true,
		},
		{
			name: "valid 6 decimals config",
			coinInfo: EvmCoinInfo{
				Denom:         "utest",
				ExtendedDenom: "atest",
				DisplayDenom:  "test",
				Decimals:      SixDecimals,
			},
			expPass: true,
		},
		{
			name: "valid 12 decimals config",
			coinInfo: EvmCoinInfo{
				Denom:         "ptest",
				ExtendedDenom: "atest",
				DisplayDenom:  "test",
				Decimals:      TwelveDecimals,
			},
			expPass: true,
		},
		{
			name: "valid 1 decimal config",
			coinInfo: EvmCoinInfo{
				Denom:         "ctest",
				ExtendedDenom: "atest",
				DisplayDenom:  "test",
				Decimals:      OneDecimals,
			},
			expPass: true,
		},
		{
			name: "empty denom",
			coinInfo: EvmCoinInfo{
				Denom:         "",
				ExtendedDenom: "atest",
				DisplayDenom:  "test",
				Decimals:      EighteenDecimals,
			},
			expPass:     false,
			errContains: "denom cannot be empty",
		},
		{
			name: "invalid denom format - starts with number",
			coinInfo: EvmCoinInfo{
				Denom:         "1test", // starts with number
				ExtendedDenom: "1test", // same as denom to avoid 18-decimal rule conflict
				DisplayDenom:  "test",
				Decimals:      EighteenDecimals,
			},
			expPass:     false,
			errContains: "invalid denom",
		},
		{
			name: "empty extended denom",
			coinInfo: EvmCoinInfo{
				Denom:         "atest",
				ExtendedDenom: "",
				DisplayDenom:  "test",
				Decimals:      EighteenDecimals,
			},
			expPass:     false,
			errContains: "extended-denom cannot be empty",
		},
		{
			name: "invalid extended denom format - too short",
			coinInfo: EvmCoinInfo{
				Denom:         "atest",
				ExtendedDenom: "a", // too short
				DisplayDenom:  "test",
				Decimals:      SixDecimals, // use 6 decimals to avoid the 18-decimal rule
			},
			expPass:     false,
			errContains: "invalid extended-denom",
		},
		{
			name: "empty display denom",
			coinInfo: EvmCoinInfo{
				Denom:         "atest",
				ExtendedDenom: "atest",
				DisplayDenom:  "",
				Decimals:      EighteenDecimals,
			},
			expPass:     false,
			errContains: "display-denom cannot be empty",
		},
		{
			name: "invalid display denom format",
			coinInfo: EvmCoinInfo{
				Denom:         "atest",
				ExtendedDenom: "atest",
				DisplayDenom:  "test@", // invalid character
				Decimals:      EighteenDecimals,
			},
			expPass:     false,
			errContains: "invalid display-denom",
		},
		{
			name: "zero decimals",
			coinInfo: EvmCoinInfo{
				Denom:         "atest",
				ExtendedDenom: "atest",
				DisplayDenom:  "test",
				Decimals:      0,
			},
			expPass:     false,
			errContains: "decimals validation failed",
		},
		{
			name: "invalid decimals over 18",
			coinInfo: EvmCoinInfo{
				Denom:         "atest",
				ExtendedDenom: "atest",
				DisplayDenom:  "test",
				Decimals:      19,
			},
			expPass:     false,
			errContains: "decimals validation failed",
		},
		{
			name: "18 decimals with different denom and extended denom",
			coinInfo: EvmCoinInfo{
				Denom:         "utest",
				ExtendedDenom: "atest", // should be same as denom for 18 decimals
				DisplayDenom:  "test",
				Decimals:      EighteenDecimals,
			},
			expPass:     false,
			errContains: "denom and extended-denom must be the same for 18 decimals",
		},
		{
			name: "6 decimals with different denom and extended denom (valid)",
			coinInfo: EvmCoinInfo{
				Denom:         "utest",
				ExtendedDenom: "atest", // different is ok for non-18 decimals
				DisplayDenom:  "test",
				Decimals:      SixDecimals,
			},
			expPass: true,
		},
		{
			name: "valid with special characters in denoms",
			coinInfo: EvmCoinInfo{
				Denom:         "test-coin",
				ExtendedDenom: "atest-coin",
				DisplayDenom:  "testcoin",
				Decimals:      SixDecimals,
			},
			expPass: true,
		},
		{
			name: "denom with numbers (valid)",
			coinInfo: EvmCoinInfo{
				Denom:         "test123",
				ExtendedDenom: "atest123",
				DisplayDenom:  "test123",
				Decimals:      SixDecimals,
			},
			expPass: true,
		},
	}

	for _, tc := range testCases {
		tc := tc //nolint:copyloopvar // Needed to work correctly with concurrent tests

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.coinInfo.Validate()

			if tc.expPass {
				require.NoError(t, err, "expected validation to pass for %s", tc.name)
			} else {
				require.Error(t, err, "expected validation to fail for %s", tc.name)
				require.Contains(t, err.Error(), tc.errContains, "error message should contain expected text")
			}
		})
	}
}

// TestEvmCoinInfoValidateEdgeCases tests additional edge cases
func TestEvmCoinInfoValidateEdgeCases(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		coinInfo EvmCoinInfo
		expPass  bool
	}{
		{
			name: "maximum valid decimals",
			coinInfo: EvmCoinInfo{
				Denom:         "atest",
				ExtendedDenom: "btest",
				DisplayDenom:  "test",
				Decimals:      SeventeenDecimals, // 17 is max non-18
			},
			expPass: true,
		},
		{
			name: "minimum valid decimals",
			coinInfo: EvmCoinInfo{
				Denom:         "atest",
				ExtendedDenom: "btest",
				DisplayDenom:  "test",
				Decimals:      OneDecimals,
			},
			expPass: true,
		},
	}

	for _, tc := range testCases {
		tc := tc //nolint:copyloopvar // Needed to work correctly with concurrent tests

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.coinInfo.Validate()

			if tc.expPass {
				require.NoError(t, err, "expected validation to pass for %s", tc.name)
			} else {
				require.Error(t, err, "expected validation to fail for %s", tc.name)
			}
		})
	}
}
