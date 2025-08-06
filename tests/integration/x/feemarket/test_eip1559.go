package feemarket

import (
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/cosmos/evm/testutil/integration/evm/network"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperTestSuite) TestCalculateBaseFee() {
	var (
		nw             *network.UnitTestNetwork
		ctx            sdk.Context
		initialBaseFee math.LegacyDec
	)

	testCases := []struct {
		name                 string
		NoBaseFee            bool
		blockHeight          int64
		parentBlockGasWanted uint64
		minGasPrice          math.LegacyDec
		expFee               func() math.LegacyDec
	}{
		{
			"without BaseFee",
			true,
			0,
			0,
			math.LegacyZeroDec(),
			nil,
		},
		{
			"with BaseFee - initial EIP-1559 block",
			false,
			0,
			0,
			math.LegacyZeroDec(),
			func() math.LegacyDec { return nw.App.GetFeeMarketKeeper().GetParams(ctx).BaseFee },
		},
		{
			"with BaseFee - parent block wanted the same gas as its target (ElasticityMultiplier = 2)",
			false,
			1,
			50,
			math.LegacyZeroDec(),
			func() math.LegacyDec { return nw.App.GetFeeMarketKeeper().GetParams(ctx).BaseFee },
		},
		{
			"with BaseFee - parent block wanted the same gas as its target, with higher min gas price (ElasticityMultiplier = 2)",
			false,
			1,
			50,
			math.LegacyNewDec(1500000000),
			func() math.LegacyDec { return nw.App.GetFeeMarketKeeper().GetParams(ctx).BaseFee },
		},
		{
			"with BaseFee - parent block wanted more gas than its target (ElasticityMultiplier = 2)",
			false,
			1,
			100,
			math.LegacyZeroDec(),
			func() math.LegacyDec { return initialBaseFee.Add(math.LegacyNewDec(109375000)) },
		},
		{
			"with BaseFee - parent block wanted more gas than its target, with higher min gas price (ElasticityMultiplier = 2)",
			false,
			1,
			100,
			math.LegacyNewDec(1500000000),
			func() math.LegacyDec { return initialBaseFee.Add(math.LegacyNewDec(109375000)) },
		},
		{
			"with BaseFee - Parent gas wanted smaller than parent gas target (ElasticityMultiplier = 2)",
			false,
			1,
			25,
			math.LegacyZeroDec(),
			func() math.LegacyDec { return initialBaseFee.Sub(math.LegacyNewDec(54687500)) },
		},
		{
			"with BaseFee - Parent gas wanted smaller than parent gas target, with higher min gas price (ElasticityMultiplier = 2)",
			false,
			1,
			25,
			math.LegacyNewDec(1500000000),
			func() math.LegacyDec { return math.LegacyNewDec(1500000000) },
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// reset network and context
			nw = network.NewUnitTestNetwork(s.create, s.options...)
			ctx = nw.GetContext()

			params := nw.App.GetFeeMarketKeeper().GetParams(ctx)
			params.NoBaseFee = tc.NoBaseFee
			params.MinGasPrice = tc.minGasPrice
			err := nw.App.GetFeeMarketKeeper().SetParams(ctx, params)
			s.NoError(err)

			initialBaseFee = params.BaseFee

			// Set block height
			ctx = ctx.WithBlockHeight(tc.blockHeight)

			// Set parent block gas
			nw.App.GetFeeMarketKeeper().SetBlockGasWanted(ctx, tc.parentBlockGasWanted)

			// Set next block target/gasLimit through Consensus Param MaxGas
			blockParams := tmproto.BlockParams{
				MaxGas:   100,
				MaxBytes: 10,
			}
			consParams := tmproto.ConsensusParams{Block: &blockParams}
			ctx = ctx.WithConsensusParams(consParams)

			fee := nw.App.GetFeeMarketKeeper().CalculateBaseFee(ctx)
			if tc.NoBaseFee {
				s.True(fee.IsNil(), tc.name)
			} else {
				s.Equal(tc.expFee(), fee, tc.name)
			}
		})
	}
}
