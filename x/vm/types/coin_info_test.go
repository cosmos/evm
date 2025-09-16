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
				DisplayDenom:     "test",
				Decimals:         EighteenDecimals,
				ExtendedDecimals: EighteenDecimals,
			},
			expPass: true,
		},
		{
			name: "valid 6 decimals config",
			coinInfo: EvmCoinInfo{
				DisplayDenom:     "test",
				Decimals:         SixDecimals,
				ExtendedDecimals: EighteenDecimals,
			},
			expPass: true,
		},
		{
			name: "valid 12 decimals config",
			coinInfo: EvmCoinInfo{
				DisplayDenom:     "test",
				Decimals:         TwelveDecimals,
				ExtendedDecimals: EighteenDecimals,
			},
			expPass: true,
		},
		{
			name: "valid 1 decimal config",
			coinInfo: EvmCoinInfo{
				DisplayDenom:     "test",
				Decimals:         OneDecimals,
				ExtendedDecimals: EighteenDecimals,
			},
			expPass: true,
		},
		{
			name: "empty display denom",
			coinInfo: EvmCoinInfo{
				DisplayDenom:     "",
				Decimals:         EighteenDecimals,
				ExtendedDecimals: EighteenDecimals,
			},
			expPass:     false,
			errContains: "invalid denom: a",
		},
		{
			name: "invalid denom format - starts with number",
			coinInfo: EvmCoinInfo{
				DisplayDenom:     "1test",
				Decimals:         EighteenDecimals,
				ExtendedDecimals: EighteenDecimals,
			},
			expPass:     false,
			errContains: "invalid denom",
		},
		{
			name: "invalid extended denom format - too short",
			coinInfo: EvmCoinInfo{
				DisplayDenom:     "t",
				Decimals:         SixDecimals,
				ExtendedDecimals: EighteenDecimals,
			},
			expPass:     false,
			errContains: "invalid denom: ut",
		},
		{
			name: "invalid display denom character",
			coinInfo: EvmCoinInfo{
				DisplayDenom:     "test@",
				Decimals:         EighteenDecimals,
				ExtendedDecimals: EighteenDecimals,
			},
			expPass:     false,
			errContains: "invalid denom: atest@",
		},
		{
			name: "zero decimals is invalid",
			coinInfo: EvmCoinInfo{
				DisplayDenom:     "test",
				Decimals:         0,
				ExtendedDecimals: EighteenDecimals,
			},
			expPass:     false,
			errContains: "decimals validation failed",
		},
		{
			name: "invalid si decimals",
			coinInfo: EvmCoinInfo{
				DisplayDenom:     "test",
				Decimals:         TenDecimals,
				ExtendedDecimals: TenDecimals,
			},
			expPass:     false,
			errContains: "received unsupported decimals: 10",
		},
		{
			name: "decimals out of valid range",
			coinInfo: EvmCoinInfo{
				DisplayDenom:     "test",
				Decimals:         Decimals(19),
				ExtendedDecimals: Decimals(19),
			},
			expPass:     false,
			errContains: "received unsupported decimals",
		},
		{
			name: "18 decimals with different extended decimals",
			coinInfo: EvmCoinInfo{
				DisplayDenom:     "test",
				Decimals:         EighteenDecimals,
				ExtendedDecimals: TwelveDecimals,
			},
			expPass:     false,
			errContains: "decimals and extended decimals must be the same for 18 decimals",
		},
		{
			name: "valid 6 decimals with different extended decimals",
			coinInfo: EvmCoinInfo{
				DisplayDenom:     "test",
				Decimals:         SixDecimals,
				ExtendedDecimals: NineDecimals,
			},
			expPass: true,
		},
		{
			name: "display denom with valid special characters",
			coinInfo: EvmCoinInfo{
				DisplayDenom:     "test-coin",
				Decimals:         SixDecimals,
				ExtendedDecimals: EighteenDecimals,
			},
			expPass: true,
		},
		{
			name: "display denom with valid numbers",
			coinInfo: EvmCoinInfo{
				DisplayDenom:     "test123",
				Decimals:         SixDecimals,
				ExtendedDecimals: EighteenDecimals,
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
