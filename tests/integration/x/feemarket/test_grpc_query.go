package feemarket

import (
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/x/feemarket/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperTestSuite) TestQueryParams() {
	var (
		nw  *network.UnitTestNetwork
		ctx sdk.Context
	)

	testCases := []struct {
		name    string
		expPass bool
	}{
		{
			"pass",
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// reset network and context
			nw = network.NewUnitTestNetwork(s.create, s.options...)
			ctx = nw.GetContext()
			qc := nw.GetFeeMarketClient()

			params := nw.App.GetFeeMarketKeeper().GetParams(ctx)
			exp := &types.QueryParamsResponse{Params: params}

			res, err := qc.Params(ctx.Context(), &types.QueryParamsRequest{})
			if tc.expPass {
				s.Equal(exp, res, tc.name)
				s.NoError(err)
			} else {
				s.Error(err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestQueryBaseFee() {
	var (
		expRes         *types.QueryBaseFeeResponse
		nw             *network.UnitTestNetwork
		ctx            sdk.Context
		initialBaseFee sdkmath.LegacyDec
	)

	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"pass - default Base Fee",
			func() {
				expRes = &types.QueryBaseFeeResponse{BaseFee: &initialBaseFee}
			},
			true,
		},
		{
			"pass - non-nil Base Fee",
			func() {
				baseFee := sdkmath.LegacyNewDec(1)
				nw.App.GetFeeMarketKeeper().SetBaseFee(ctx, baseFee)

				expRes = &types.QueryBaseFeeResponse{BaseFee: &baseFee}
			},
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// reset network and context
			nw = network.NewUnitTestNetwork(s.create, s.options...)
			ctx = nw.GetContext()
			qc := nw.GetFeeMarketClient()
			initialBaseFee = nw.App.GetFeeMarketKeeper().GetBaseFee(ctx)

			tc.malleate()

			res, err := qc.BaseFee(ctx.Context(), &types.QueryBaseFeeRequest{})
			if tc.expPass {
				s.NotNil(res)
				s.Equal(expRes, res, tc.name)
				s.NoError(err)
			} else {
				s.Error(err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestQueryBlockGas() {
	var (
		nw  *network.UnitTestNetwork
		ctx sdk.Context
	)
	testCases := []struct {
		name    string
		expPass bool
	}{
		{
			"pass",
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// reset network and context
			nw = network.NewUnitTestNetwork(s.create, s.options...)
			ctx = nw.GetContext()
			qc := nw.GetFeeMarketClient()

			gas := nw.App.GetFeeMarketKeeper().GetBlockGasWanted(ctx)
			exp := &types.QueryBlockGasResponse{Gas: int64(gas)} //#nosec G115

			res, err := qc.BlockGas(ctx.Context(), &types.QueryBlockGasRequest{})
			if tc.expPass {
				s.Equal(exp, res, tc.name)
				s.NoError(err)
			} else {
				s.Error(err)
			}
		})
	}
}
