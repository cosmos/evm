package erc20

import (
	"fmt"

	"github.com/cosmos/evm/testutil/config"
	testconstants "github.com/cosmos/evm/testutil/constants"
	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/erc20/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
)

func (s *KeeperTestSuite) TestTokenPairs() {
	var (
		ctx    sdk.Context
		req    *types.QueryTokenPairsRequest
		expRes *types.QueryTokenPairsResponse
	)

	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"no pairs registered",
			func() {
				req = &types.QueryTokenPairsRequest{}
				expRes = &types.QueryTokenPairsResponse{
					Pagination: &query.PageResponse{
						Total: 1,
					},
					TokenPairs: testconstants.ExampleTokenPairs,
				}
			},
			true,
		},
		{
			"1 pair registered w/pagination",
			func() {
				req = &types.QueryTokenPairsRequest{
					Pagination: &query.PageRequest{Limit: 10, CountTotal: true},
				}
				pairs := testconstants.ExampleTokenPairs
				pair := types.NewTokenPair(utiltx.GenerateAddress(), "coin", types.OWNER_MODULE)
				s.network.App.GetErc20Keeper().SetTokenPair(ctx, pair)
				pairs = append(pairs, pair)

				expRes = &types.QueryTokenPairsResponse{
					Pagination: &query.PageResponse{Total: uint64(len(pairs))},
					TokenPairs: pairs,
				}
			},
			true,
		},
		{
			"2 pairs registered wo/pagination",
			func() {
				req = &types.QueryTokenPairsRequest{}
				pairs := testconstants.ExampleTokenPairs

				pair := types.NewTokenPair(utiltx.GenerateAddress(), "coin", types.OWNER_MODULE)
				pair2 := types.NewTokenPair(utiltx.GenerateAddress(), "coin2", types.OWNER_MODULE)
				s.network.App.GetErc20Keeper().SetTokenPair(ctx, pair)
				s.network.App.GetErc20Keeper().SetTokenPair(ctx, pair2)
				pairs = append(pairs, pair, pair2)

				expRes = &types.QueryTokenPairsResponse{
					Pagination: &query.PageResponse{Total: uint64(len(pairs))},
					TokenPairs: pairs,
				}
			},
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()

			tc.malleate()

			res, err := s.queryClient.TokenPairs(ctx, req)
			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(expRes.Pagination, res.Pagination)
				s.Require().ElementsMatch(expRes.TokenPairs, res.TokenPairs)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestTokenPair() {
	var (
		ctx    sdk.Context
		req    *types.QueryTokenPairRequest
		expRes *types.QueryTokenPairResponse
	)

	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"invalid token address",
			func() {
				req = &types.QueryTokenPairRequest{}
				expRes = &types.QueryTokenPairResponse{}
			},
			false,
		},
		{
			"token pair not found",
			func() {
				req = &types.QueryTokenPairRequest{
					Token: utiltx.GenerateAddress().Hex(),
				}
				expRes = &types.QueryTokenPairResponse{}
			},
			false,
		},
		{
			"token pair found",
			func() {
				addr := utiltx.GenerateAddress()
				pair := types.NewTokenPair(addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
				req = &types.QueryTokenPairRequest{
					Token: pair.Erc20Address,
				}
				expRes = &types.QueryTokenPairResponse{TokenPair: pair}
			},
			true,
		},
		{
			"token pair not found - with erc20 existent",
			func() {
				addr := utiltx.GenerateAddress()
				pair := types.NewTokenPair(addr, "coin", types.OWNER_MODULE)
				s.network.App.GetErc20Keeper().SetERC20Map(ctx, addr, pair.GetID())
				s.network.App.GetErc20Keeper().SetDenomMap(ctx, pair.Denom, pair.GetID())

				req = &types.QueryTokenPairRequest{
					Token: pair.Erc20Address,
				}
				expRes = &types.QueryTokenPairResponse{TokenPair: pair}
			},
			false,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()

			tc.malleate()

			res, err := s.queryClient.TokenPair(ctx, req)
			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(expRes, res)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestQueryParams() {
	s.SetupTest()
	ctx := s.network.GetContext()
	expParams := config.NewErc20GenesisState().Params

	res, err := s.queryClient.Params(ctx, &types.QueryParamsRequest{})
	s.Require().NoError(err)
	s.Require().Equal(expParams, res.Params)
}
