package feemarket

import (
	"github.com/cosmos/evm/testutil/integration/evm/network"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperTestSuite) TestSetGetBlockGasWanted() {
	var (
		nw  *network.UnitTestNetwork
		ctx sdk.Context
	)
	testCases := []struct {
		name     string
		malleate func()
		expGas   uint64
	}{
		{
			"with last block given",
			func() {
				nw.App.GetFeeMarketKeeper().SetBlockGasWanted(ctx, uint64(1000000))
			},
			uint64(1000000),
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// reset network and context
			nw = network.NewUnitTestNetwork(s.create, s.options...)
			ctx = nw.GetContext()

			tc.malleate()

			gas := nw.App.GetFeeMarketKeeper().GetBlockGasWanted(ctx)
			s.Equal(tc.expGas, gas, tc.name)
		})
	}
}

func (s *KeeperTestSuite) TestSetGetGasFee() {
	var (
		nw  *network.UnitTestNetwork
		ctx sdk.Context
	)
	testCases := []struct {
		name     string
		malleate func()
		expFee   math.LegacyDec
	}{
		{
			"with last block given",
			func() {
				nw.App.GetFeeMarketKeeper().SetBaseFee(ctx, math.LegacyOneDec())
			},
			math.LegacyOneDec(),
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// reset network and context
			nw = network.NewUnitTestNetwork(s.create, s.options...)
			ctx = nw.GetContext()

			tc.malleate()

			fee := nw.App.GetFeeMarketKeeper().GetBaseFee(ctx)
			s.Equal(tc.expFee, fee, tc.name)
		})
	}
}
