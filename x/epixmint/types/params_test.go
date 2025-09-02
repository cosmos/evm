package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"

	"github.com/cosmos/evm/x/epixmint/types"
)

func TestDefaultParams(t *testing.T) {
	params := types.DefaultParams()

	require.Equal(t, "aepix", params.MintDenom)
	require.Equal(t, uint64(6), params.BlockTimeSeconds)
	require.True(t, params.InitialAnnualMintAmount.IsPositive())
	require.True(t, params.MaxSupply.IsPositive())
	require.True(t, params.MaxSupply.GT(params.InitialAnnualMintAmount))
	require.Equal(t, math.LegacyMustNewDecFromStr("0.25"), params.AnnualReductionRate)
}

func TestParamsValidate(t *testing.T) {
	testCases := []struct {
		name     string
		params   types.Params
		expError bool
	}{
		{
			name:     "default params",
			params:   types.DefaultParams(),
			expError: false,
		},
		{
			name: "invalid mint denom",
			params: types.Params{
				MintDenom:               "",
				InitialAnnualMintAmount: math.NewInt(1000),
				AnnualReductionRate:     math.LegacyMustNewDecFromStr("0.25"),
				BlockTimeSeconds:        6,
				MaxSupply:               math.NewInt(10000),
				CommunityPoolRate:       math.LegacyMustNewDecFromStr("0.02"),
				StakingRewardsRate:      math.LegacyMustNewDecFromStr("0.98"),
			},
			expError: true,
		},
		{
			name: "negative initial annual mint amount",
			params: types.Params{
				MintDenom:               "aepix",
				InitialAnnualMintAmount: math.NewInt(-1000),
				AnnualReductionRate:     math.LegacyMustNewDecFromStr("0.25"),
				BlockTimeSeconds:        6,
				MaxSupply:               math.NewInt(10000),
				CommunityPoolRate:       math.LegacyMustNewDecFromStr("0.02"),
				StakingRewardsRate:      math.LegacyMustNewDecFromStr("0.98"),
			},
			expError: true,
		},
		{
			name: "negative max supply",
			params: types.Params{
				MintDenom:               "aepix",
				InitialAnnualMintAmount: math.NewInt(1000),
				AnnualReductionRate:     math.LegacyMustNewDecFromStr("0.25"),
				BlockTimeSeconds:        6,
				MaxSupply:               math.NewInt(-10000),
				CommunityPoolRate:       math.LegacyMustNewDecFromStr("0.02"),
				StakingRewardsRate:      math.LegacyMustNewDecFromStr("0.98"),
			},
			expError: true,
		},
		{
			name: "zero block time seconds",
			params: types.Params{
				MintDenom:               "aepix",
				InitialAnnualMintAmount: math.NewInt(1000),
				AnnualReductionRate:     math.LegacyMustNewDecFromStr("0.25"),
				BlockTimeSeconds:        0,
				MaxSupply:               math.NewInt(10000),
				CommunityPoolRate:       math.LegacyMustNewDecFromStr("0.02"),
				StakingRewardsRate:      math.LegacyMustNewDecFromStr("0.98"),
			},
			expError: true,
		},
		{
			name: "valid custom params",
			params: types.Params{
				MintDenom:               "uepix",
				InitialAnnualMintAmount: math.NewInt(1000000),
				AnnualReductionRate:     math.LegacyMustNewDecFromStr("0.25"),
				BlockTimeSeconds:        5, // 5 second blocks
				MaxSupply:               math.NewInt(100000000),
				CommunityPoolRate:       math.LegacyMustNewDecFromStr("0.02"),
				StakingRewardsRate:      math.LegacyMustNewDecFromStr("0.98"),
			},
			expError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()
			if tc.expError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParamsString(t *testing.T) {
	params := types.DefaultParams()
	str := params.String()

	require.Contains(t, str, "Mint Params:")
	require.Contains(t, str, "aepix")
	require.Contains(t, str, "10527000000000000000000000000") // Initial annual mint amount
	require.Contains(t, str, "0.250000000000000000")          // Annual reduction rate
	require.Contains(t, str, "6")                             // Block time seconds
}
