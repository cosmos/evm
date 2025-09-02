package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"

	"github.com/cosmos/evm/x/epixmint/types"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

func TestMsgUpdateParams_ValidateBasic(t *testing.T) {
	testCases := []struct {
		name      string
		msg       *types.MsgUpdateParams
		expectErr bool
	}{
		{
			name: "valid message",
			msg: &types.MsgUpdateParams{
				Authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
				Params: types.Params{
					MintDenom:               "aepix",
					InitialAnnualMintAmount: math.NewInt(1000000),
					AnnualReductionRate:     math.LegacyMustNewDecFromStr("0.25"),
					BlockTimeSeconds:        6,
					MaxSupply:               math.NewInt(100000000),
					CommunityPoolRate:       math.LegacyMustNewDecFromStr("0.02"),
					StakingRewardsRate:      math.LegacyMustNewDecFromStr("0.98"),
				},
			},
			expectErr: false,
		},
		{
			name: "invalid authority",
			msg: &types.MsgUpdateParams{
				Authority: "invalid-address",
				Params: types.Params{
					MintDenom:               "aepix",
					InitialAnnualMintAmount: math.NewInt(1000000),
					AnnualReductionRate:     math.LegacyMustNewDecFromStr("0.25"),
					BlockTimeSeconds:        6,
					MaxSupply:               math.NewInt(100000000),
					CommunityPoolRate:       math.LegacyMustNewDecFromStr("0.02"),
					StakingRewardsRate:      math.LegacyMustNewDecFromStr("0.98"),
				},
			},
			expectErr: true,
		},
		{
			name: "invalid params - empty denom",
			msg: &types.MsgUpdateParams{
				Authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
				Params: types.Params{
					MintDenom:               "", // Invalid
					InitialAnnualMintAmount: math.NewInt(1000000),
					AnnualReductionRate:     math.LegacyMustNewDecFromStr("0.25"),
					BlockTimeSeconds:        6,
					MaxSupply:               math.NewInt(100000000),
					CommunityPoolRate:       math.LegacyMustNewDecFromStr("0.02"),
					StakingRewardsRate:      math.LegacyMustNewDecFromStr("0.98"),
				},
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgUpdateParams_GetSigners(t *testing.T) {
	authority := authtypes.NewModuleAddress(govtypes.ModuleName)
	msg := &types.MsgUpdateParams{
		Authority: authority.String(),
		Params:    types.DefaultParams(),
	}

	signers := msg.GetSigners()
	require.Len(t, signers, 1)
	require.Equal(t, authority, signers[0])
}
