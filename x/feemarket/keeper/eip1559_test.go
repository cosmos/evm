package keeper_test

import (
	"testing"

	math "math"

	"github.com/stretchr/testify/require"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/cosmos/evm/testutil/integration/os/network"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestCalculateBaseFee(t *testing.T) {
	var (
		nw             *network.UnitTestNetwork
		ctx            sdk.Context
		initialBaseFee sdkmath.LegacyDec
	)

	testCases := []struct {
		name        string
		NoBaseFee   bool
		blockHeight int64
		gasUsed uint64
		minGasPrice sdkmath.LegacyDec
		expFee      func() sdkmath.LegacyDec
	}{
		{
			"without BaseFee",
			true,
			0,
			0,
			sdkmath.LegacyZeroDec(),
			nil,
		},
		{
			"with BaseFee - initial EIP-1559 block",
			false,
			0,
			0,
			sdkmath.LegacyZeroDec(),
			func() sdkmath.LegacyDec { return nw.App.FeeMarketKeeper.GetParams(ctx).BaseFee },
		},
		{
			"with BaseFee - parent block wanted the same gas as its target (ElasticityMultiplier = 2)",
			false,
			1,
			50,
			sdkmath.LegacyZeroDec(),
			func() sdkmath.LegacyDec { return nw.App.FeeMarketKeeper.GetParams(ctx).BaseFee },
		},
		{
			"with BaseFee - parent block wanted the same gas as its target, with higher min gas price (ElasticityMultiplier = 2)",
			false,
			1,
			50,
			sdkmath.LegacyNewDec(1500000000),
			func() sdkmath.LegacyDec { return nw.App.FeeMarketKeeper.GetParams(ctx).BaseFee }, // Base fee remains unchanged when gas used equals target, min gas price not applied
		},
		{
			"with BaseFee - parent block wanted more gas than its target (ElasticityMultiplier = 2)",
			false,
			1,
			100,
			sdkmath.LegacyZeroDec(),
			func() sdkmath.LegacyDec { return initialBaseFee.Add(sdkmath.LegacyNewDec(109375000)) },
		},
		{
			"with BaseFee - parent block wanted more gas than its target, with higher min gas price (ElasticityMultiplier = 2)",
			false,
			1,
			100,
			sdkmath.LegacyNewDec(1500000000),
			func() sdkmath.LegacyDec { return initialBaseFee.Add(sdkmath.LegacyNewDec(109375000)) },
		},
		{
			"with BaseFee - Parent gas wanted smaller than parent gas target (ElasticityMultiplier = 2)",
			false,
			1,
			25,
			sdkmath.LegacyZeroDec(),
			func() sdkmath.LegacyDec { return initialBaseFee.Sub(sdkmath.LegacyNewDec(54687500)) },
		},
		{
			"with BaseFee - Parent gas wanted smaller than parent gas target, with higher min gas price (ElasticityMultiplier = 2)",
			false,
			1,
			25,
			sdkmath.LegacyNewDec(1500000000),
			func() sdkmath.LegacyDec { return sdkmath.LegacyNewDec(1500000000) },
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// reset network and context
			nw = network.NewUnitTestNetwork()
			ctx = nw.GetContext()

			params := nw.App.FeeMarketKeeper.GetParams(ctx)
			params.NoBaseFee = tc.NoBaseFee
			params.MinGasPrice = tc.minGasPrice
			err := nw.App.FeeMarketKeeper.SetParams(ctx, params)
			require.NoError(t, err)

			initialBaseFee = params.BaseFee

			// Set block height
			ctx = ctx.WithBlockHeight(tc.blockHeight)
			meter := storetypes.NewGasMeter(uint64(1000000000))
			ctx = ctx.WithBlockGasMeter(meter)
			ctx.BlockGasMeter().ConsumeGas(tc.gasUsed, "test")
			nw.App.FeeMarketKeeper.SetTransientBlockGasWanted(ctx, tc.gasUsed) // Set transient gas wanted to match gas used

			// Set block target/gasLimit through Consensus Param MaxGas
			blockParams := tmproto.BlockParams{
				MaxGas:   100,
				MaxBytes: 10,
			}
			consParams := tmproto.ConsensusParams{Block: &blockParams}
			ctx = ctx.WithConsensusParams(consParams)


			// Run EndBlock to calculate base fee
			err = nw.App.FeeMarketKeeper.EndBlock(ctx)
			require.NoError(t, err)

			fee := nw.App.FeeMarketKeeper.GetBaseFee(ctx)
			if tc.NoBaseFee {
				require.True(t, fee.IsNil(), tc.name)
			} else {
				require.Equal(t, tc.expFee(), fee, tc.name)
			}
		})
	}
}

func TestCalculateBlockGasWanted(t *testing.T) {
	var (
		nw  *network.UnitTestNetwork
		ctx sdk.Context
	)

	testCases := []struct {
		name         string
		malleate     func()
		expGasWanted uint64
		expError     bool
	}{
		{
			"nil block gas meter",
			func() {
				ctx = ctx.WithBlockGasMeter(nil)
			},
			0,
			true,
		},
		{
			"gas wanted overflow",
			func() {
				meter := storetypes.NewGasMeter(uint64(1000000000))
				ctx = ctx.WithBlockGasMeter(meter)
				// Set gas wanted to max uint64 to cause overflow
				nw.App.FeeMarketKeeper.SetTransientBlockGasWanted(ctx, math.MaxUint64)
			},
			0,
			true,
		},
		{
			"gas used overflow",
			func() {
				meter := storetypes.NewGasMeter(math.MaxUint64)
				ctx = ctx.WithBlockGasMeter(meter)
				// Consume max gas to cause overflow
				ctx.BlockGasMeter().ConsumeGas(math.MaxUint64, "test")
				nw.App.FeeMarketKeeper.SetTransientBlockGasWanted(ctx, 1000000)
			},
			0,
			true,
		},
		{
			"gas used less than gas wanted",
			func() {
				meter := storetypes.NewGasMeter(uint64(1000000000))
				ctx = ctx.WithBlockGasMeter(meter)
				// Set gas used to be less than gas wanted
				ctx.BlockGasMeter().ConsumeGas(1000000, "test")
				nw.App.FeeMarketKeeper.SetTransientBlockGasWanted(ctx, 5000000)
			},
			uint64(2500000), // 5000000 * 0.5 (MinGasMultiplier)
			false,
		},
		{
			"gas used more than gas wanted",
			func() {
				meter := storetypes.NewGasMeter(uint64(1000000000))
				ctx = ctx.WithBlockGasMeter(meter)
				// Set gas used higher than gas wanted
				ctx.BlockGasMeter().ConsumeGas(3000000, "test")
				nw.App.FeeMarketKeeper.SetTransientBlockGasWanted(ctx, 2000000)
			},
			uint64(3000000), // Should use gas used as it's higher than gas wanted * multiplier
			false,
		},
		{
			"gas used equals gas wanted",
			func() {
				meter := storetypes.NewGasMeter(uint64(1000000000))
				ctx = ctx.WithBlockGasMeter(meter)
				// Set gas used equal to gas wanted
				ctx.BlockGasMeter().ConsumeGas(2000000, "test")
				nw.App.FeeMarketKeeper.SetTransientBlockGasWanted(ctx, 2000000)
			},
			uint64(2000000), // Should use gas used as it equals gas wanted
			false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// reset network and context
			nw = network.NewUnitTestNetwork()
			ctx = nw.GetContext()

			tc.malleate()

			gasWanted, err := nw.App.FeeMarketKeeper.CalculateBlockGasWanted(ctx)
			if tc.expError {
				require.Error(t, err, tc.name)
				return
			}
			require.NoError(t, err, tc.name)
			require.Equal(t, tc.expGasWanted, gasWanted, tc.name)
		})
	}
}
