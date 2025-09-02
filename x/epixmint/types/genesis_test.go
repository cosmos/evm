package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"

	"github.com/cosmos/evm/x/epixmint/types"
)

func TestDefaultGenesisState(t *testing.T) {
	genState := types.DefaultGenesisState()
	
	require.NotNil(t, genState)
	require.Equal(t, types.DefaultParams(), genState.Params)
}

func TestNewGenesisState(t *testing.T) {
	params := types.DefaultParams()
	genState := types.NewGenesisState(params)
	
	require.NotNil(t, genState)
	require.Equal(t, params, genState.Params)
}

func TestGenesisStateValidate(t *testing.T) {
	testCases := []struct {
		name     string
		genState *types.GenesisState
		expError bool
	}{
		{
			name:     "default genesis state",
			genState: types.DefaultGenesisState(),
			expError: false,
		},
		{
			name: "valid custom genesis state",
			genState: types.NewGenesisState(types.Params{
				MintDenom:               "uepix",
				InitialAnnualMintAmount: math.NewInt(1000000),
				AnnualReductionRate:     math.LegacyMustNewDecFromStr("0.25"),
				BlockTimeSeconds:        6,
				MaxSupply:               math.NewInt(100000000),
				CommunityPoolRate:       math.LegacyMustNewDecFromStr("0.02"),
				StakingRewardsRate:      math.LegacyMustNewDecFromStr("0.98"),
			}),
			expError: false,
		},
		{
			name: "invalid params in genesis state",
			genState: types.NewGenesisState(types.Params{
				MintDenom:               "", // Invalid empty denom
				InitialAnnualMintAmount: math.NewInt(1000000),
				AnnualReductionRate:     math.LegacyMustNewDecFromStr("0.25"),
				BlockTimeSeconds:        6,
				MaxSupply:               math.NewInt(100000000),
				CommunityPoolRate:       math.LegacyMustNewDecFromStr("0.02"),
				StakingRewardsRate:      math.LegacyMustNewDecFromStr("0.98"),
			}),
			expError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.expError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
