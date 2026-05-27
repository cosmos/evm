package feemarket

import (
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/x/feemarket/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

func (s *KeeperTestSuite) TestUpdateParams() {
	var (
		nw  *network.UnitTestNetwork
		ctx sdk.Context
	)

	testCases := []struct {
		name      string
		request   *types.MsgUpdateParams
		expectErr bool
	}{
		{
			name:      "fail - invalid authority",
			request:   &types.MsgUpdateParams{Authority: "foobar"},
			expectErr: true,
		},
		{
			name: "pass - valid Update msg",
			request: &types.MsgUpdateParams{
				Authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
				Params:    types.DefaultParams(),
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// reset network and context
			nw = network.NewUnitTestNetwork(s.create, s.options...)
			ctx = nw.GetContext()

			_, err := nw.App.GetFeeMarketKeeper().UpdateParams(ctx, tc.request)
			if tc.expectErr {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}

// TestUpdateParamsAuthority verifies that the feemarket keeper resolves the
// authority through the consensus AuthorityParams when set, and otherwise
// falls back to the keeper's authority.
func (s *KeeperTestSuite) TestUpdateParamsAuthority() {
	nw := network.NewUnitTestNetwork(s.create, s.options...)

	keeperAuthority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	overrideAuthority := sdk.AccAddress("override_authority___").String()
	s.Require().NotEqual(keeperAuthority, overrideAuthority)

	s.Run("fallback to keeper authority when consensus authority is unset", func() {
		ctx := nw.GetContext()

		_, err := nw.App.GetFeeMarketKeeper().UpdateParams(ctx, &types.MsgUpdateParams{
			Authority: keeperAuthority,
			Params:    types.DefaultParams(),
		})
		s.Require().NoError(err)

		_, err = nw.App.GetFeeMarketKeeper().UpdateParams(ctx, &types.MsgUpdateParams{
			Authority: overrideAuthority,
			Params:    types.DefaultParams(),
		})
		s.Require().Error(err)
		s.Require().Contains(err.Error(), "invalid authority")
	})

	s.Run("consensus authority takes precedence over keeper authority", func() {
		ctx := nw.GetContext().WithConsensusParams(cmtproto.ConsensusParams{
			Authority: &cmtproto.AuthorityParams{Authority: overrideAuthority},
		})

		_, err := nw.App.GetFeeMarketKeeper().UpdateParams(ctx, &types.MsgUpdateParams{
			Authority: overrideAuthority,
			Params:    types.DefaultParams(),
		})
		s.Require().NoError(err)

		_, err = nw.App.GetFeeMarketKeeper().UpdateParams(ctx, &types.MsgUpdateParams{
			Authority: keeperAuthority,
			Params:    types.DefaultParams(),
		})
		s.Require().Error(err)
		s.Require().Contains(err.Error(), "invalid authority")
	})
}
