package feemarket

import (
	"github.com/cosmos/evm/testutil/integration/evm/network"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperTestSuite) TestEndBlock() {
	var (
		nw  *network.UnitTestNetwork
		ctx sdk.Context
	)

	testCases := []struct {
		name         string
		NoBaseFee    bool
		malleate     func()
		expGasWanted uint64
	}{
		{
			"baseFee nil",
			true,
			func() {},
			uint64(0),
		},
		{
			"pass",
			false,
			func() {
				meter := storetypes.NewGasMeter(uint64(1000000000))
				ctx = ctx.WithBlockGasMeter(meter).WithBlockGasWanted(5000000)
			},
			uint64(2500000),
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// reset network and context
			nw = network.NewUnitTestNetwork(s.create, s.options...)
			ctx = nw.GetContext()

			params := nw.App.GetFeeMarketKeeper().GetParams(ctx)
			params.NoBaseFee = tc.NoBaseFee

			err := nw.App.GetFeeMarketKeeper().SetParams(ctx, params)
			s.NoError(err)

			tc.malleate()

			err = nw.App.GetFeeMarketKeeper().EndBlock(ctx)
			s.NoError(err)

			gasWanted := nw.App.GetFeeMarketKeeper().GetBlockGasWanted(ctx)
			s.Equal(tc.expGasWanted, gasWanted, tc.name)
		})
	}
}
