package erc20

import (
	"fmt"

	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/erc20/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperTestSuite) TestMintingEnabled() {
	var ctx sdk.Context
	receiver := sdk.AccAddress(utiltx.GenerateAddress().Bytes())
	expPair := types.NewTokenPair(utiltx.GenerateAddress(), "coin", types.OWNER_MODULE)
	id := expPair.GetID()

	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"conversion is disabled globally",
			func() {
				params := types.DefaultParams()
				params.EnableErc20 = false
				s.network.App.GetErc20Keeper().SetParams(ctx, params) //nolint:errcheck
			},
			false,
		},
		{
			"token pair not found",
			func() {},
			false,
		},
		{
			"conversion is disabled for the given pair",
			func() {
				expPair.Enabled = false
				s.network.App.GetErc20Keeper().SetTokenPair(ctx, expPair)
				s.network.App.GetErc20Keeper().SetDenomMap(ctx, expPair.Denom, id)
				s.network.App.GetErc20Keeper().SetERC20Map(ctx, expPair.GetERC20Contract(), id)
			},
			false,
		},
		{
			"token transfers are disabled",
			func() {
				expPair.Enabled = true
				s.network.App.GetErc20Keeper().SetTokenPair(ctx, expPair)
				s.network.App.GetErc20Keeper().SetDenomMap(ctx, expPair.Denom, id)
				s.network.App.GetErc20Keeper().SetERC20Map(ctx, expPair.GetERC20Contract(), id)

				s.network.App.GetBankKeeper().SetSendEnabled(ctx, expPair.Denom, false)
			},
			false,
		},
		{
			"token not registered",
			func() {
				s.network.App.GetErc20Keeper().SetDenomMap(ctx, expPair.Denom, id)
				s.network.App.GetErc20Keeper().SetERC20Map(ctx, expPair.GetERC20Contract(), id)
			},
			false,
		},
		{
			"receiver address is blocked (module account)",
			func() {
				s.network.App.GetErc20Keeper().SetTokenPair(ctx, expPair)
				s.network.App.GetErc20Keeper().SetDenomMap(ctx, expPair.Denom, id)
				s.network.App.GetErc20Keeper().SetERC20Map(ctx, expPair.GetERC20Contract(), id)

				acc := s.network.App.GetAccountKeeper().GetModuleAccount(ctx, types.ModuleName)
				receiver = acc.GetAddress()
			},
			false,
		},
		{
			"ok",
			func() {
				s.network.App.GetErc20Keeper().SetTokenPair(ctx, expPair)
				s.network.App.GetErc20Keeper().SetDenomMap(ctx, expPair.Denom, id)
				s.network.App.GetErc20Keeper().SetERC20Map(ctx, expPair.GetERC20Contract(), id)

				receiver = sdk.AccAddress(utiltx.GenerateAddress().Bytes())
			},
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()

			tc.malleate()

			pair, err := s.network.App.GetErc20Keeper().MintingEnabled(ctx, receiver, expPair.Erc20Address)
			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(expPair, pair)
			} else {
				s.Require().Error(err)
			}
		})
	}
}
