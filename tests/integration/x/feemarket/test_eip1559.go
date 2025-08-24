package feemarket

import (
	"math/big"

	"github.com/stretchr/testify/require"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/cosmos/evm/testutil/integration"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/x/feemarket/keeper"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

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

func (s *KeeperTestSuite) TestCalculateBaseFeeEdgeCases() {
	var (
		nw  *network.UnitTestNetwork
		ctx sdk.Context
	)

	testCases := []struct {
		name           string
		blockMaxGas    int64 // MaxGas from consensus params - determines parent gas target
		setupParams    func() feemarkettypes.Params
		setupBlockData func(k *keeper.Keeper, ctx sdk.Context)
		currentBlock   int64
		parentBaseFee  *big.Int
		expectedResult *big.Int
		expectedZero   bool // For disabled/pre-London cases
		checkFunc      func(s *KeeperTestSuite, result math.LegacyDec, parentBaseFee *big.Int)
	}{
		{
			name: "EIP-1559 disabled - returns zero",
			setupParams: func() feemarkettypes.Params {
				return feemarkettypes.Params{
					NoBaseFee:                true,
					ElasticityMultiplier:     2,
					BaseFeeChangeDenominator: 8,
					MinGasPrice:              math.LegacyZeroDec(),
				}
			},
			setupBlockData: func(k *keeper.Keeper, ctx sdk.Context) {},
			currentBlock:   10,
			expectedZero:   true,
		},
		{
			name: "pre-London block - returns zero",
			setupParams: func() feemarkettypes.Params {
				return feemarkettypes.Params{
					NoBaseFee:                false,
					ElasticityMultiplier:     2,
					BaseFeeChangeDenominator: 8,
					MinGasPrice:              math.LegacyZeroDec(),
				}
			},
			setupBlockData: func(k *keeper.Keeper, ctx sdk.Context) {
				// Set London activation height to future block
				params := k.GetParams(ctx)
				params.EnableHeight = 100
				err := k.SetParams(ctx, params)
				require.NoError(s.T(), err)
			},
			currentBlock: 50,
			expectedZero: true,
		},
		{
			name: "first EIP-1559 block - returns DefaultBaseFee",
			setupParams: func() feemarkettypes.Params {
				return feemarkettypes.Params{
					NoBaseFee:                false,
					BaseFee:                  math.LegacyNewDec(1000000000),
					ElasticityMultiplier:     2,
					BaseFeeChangeDenominator: 8,
					MinGasPrice:              math.LegacyZeroDec(),
					EnableHeight:             10,
				}
			},
			setupBlockData: func(k *keeper.Keeper, ctx sdk.Context) {
				// First block, no previous base fee
			},
			currentBlock:   10,
			expectedResult: big.NewInt(1000000000),
		},
		{
			name:        "gas used equals target - base fee unchanged",
			blockMaxGas: 10000000, // parentGasTarget = 10000000 / 2 = 5000000 (ElasticityMultiplier=2)
			setupParams: func() feemarkettypes.Params {
				return feemarkettypes.Params{
					NoBaseFee:                false,
					ElasticityMultiplier:     2, // This divides MaxGas to get parentGasTarget
					BaseFeeChangeDenominator: 8,
					MinGasPrice:              math.LegacyZeroDec(),
					EnableHeight:             1,
				}
			},
			setupBlockData: func(k *keeper.Keeper, ctx sdk.Context) {
				// Set parent block gas usage equal to target
				// parentGasTarget = blockMaxGas / ElasticityMultiplier = 10000000 / 2 = 5000000
				parentGasTarget := uint64(5000000)
				k.SetBlockGasWanted(ctx, parentGasTarget) // Gas used equals target
				k.SetBaseFee(ctx, math.LegacyNewDecFromBigInt(big.NewInt(1000000000)))
			},
			currentBlock:   10,
			parentBaseFee:  big.NewInt(1000000000),
			expectedResult: big.NewInt(1000000000), // Base fee unchanged when gas used = target
		},
		{
			name:        "gas used > target - base fee increases",
			blockMaxGas: 20000000, // parentGasTarget = 20000000 / 2 = 10000000 (ElasticityMultiplier=2)
			setupParams: func() feemarkettypes.Params {
				return feemarkettypes.Params{
					NoBaseFee:                false,
					ElasticityMultiplier:     2, // This divides MaxGas to get parentGasTarget
					BaseFeeChangeDenominator: 8,
					MinGasPrice:              math.LegacyZeroDec(),
					EnableHeight:             1,
				}
			},
			setupBlockData: func(k *keeper.Keeper, ctx sdk.Context) {
				// Set parent block gas usage above target
				// parentGasTarget = blockMaxGas / ElasticityMultiplier = 20000000 / 2 = 10000000
				// Setting gasUsed = 15000000 (50% above target)
				k.SetBlockGasWanted(ctx, 15000000) // Gas used > target
				k.SetBaseFee(ctx, math.LegacyNewDecFromBigInt(big.NewInt(1000000000)))
			},
			currentBlock:  10,
			parentBaseFee: big.NewInt(1000000000),
			checkFunc: func(s *KeeperTestSuite, result math.LegacyDec, parentBaseFee *big.Int) {
				s.T().Helper()
				parentDec := math.LegacyNewDecFromBigInt(parentBaseFee)
				require.True(s.T(), result.GT(parentDec), "Base fee should increase when gas used > target")
			},
		},
		{
			name:        "gas used < target - base fee decreases",
			blockMaxGas: 20000000, // parentGasTarget = 20000000 / 2 = 10000000 (ElasticityMultiplier=2)
			setupParams: func() feemarkettypes.Params {
				return feemarkettypes.Params{
					NoBaseFee:                false,
					ElasticityMultiplier:     2, // This divides MaxGas to get parentGasTarget
					BaseFeeChangeDenominator: 8,
					MinGasPrice:              math.LegacyNewDec(1_000_000_000), // 1 minimum gas unit
					EnableHeight:             1,
				}
			},
			setupBlockData: func(k *keeper.Keeper, ctx sdk.Context) {
				// Set parent block gas usage below target
				// parentGasTarget = blockMaxGas / ElasticityMultiplier = 20000000 / 2 = 10000000
				// Setting gasUsed = 5000000 (50% below target)
				k.SetBlockGasWanted(ctx, 5000000) // Gas used < target
				k.SetBaseFee(ctx, math.LegacyNewDecFromBigInt(big.NewInt(1000000000)))
			},
			currentBlock:  10,
			parentBaseFee: big.NewInt(1000000000),
			checkFunc: func(s *KeeperTestSuite, result math.LegacyDec, parentBaseFee *big.Int) {
				s.T().Helper()
				// Should be at least min gas price
				factor := math.LegacyNewDecFromInt(evmtypes.GetEVMCoinDecimals().ConversionFactor())
				expectedMinGasPrice := math.LegacyNewDec(1_000_000_000).Mul(factor)
				require.True(s.T(), result.GTE(expectedMinGasPrice), "Result should be at least min gas price")
			},
		},
		{
			name:        "base fee decrease with low min gas price",
			blockMaxGas: 20000000, // parentGasTarget = 20000000 / 2 = 10000000 (ElasticityMultiplier=2)
			setupParams: func() feemarkettypes.Params {
				return feemarkettypes.Params{
					NoBaseFee:                false,
					ElasticityMultiplier:     2, // This divides MaxGas to get parentGasTarget
					BaseFeeChangeDenominator: 8,
					MinGasPrice:              math.LegacyNewDecWithPrec(1, 12), // Very low
					EnableHeight:             1,
				}
			},
			setupBlockData: func(k *keeper.Keeper, ctx sdk.Context) {
				// Set parent block gas usage below target
				// parentGasTarget = blockMaxGas / ElasticityMultiplier = 20000000 / 2 = 10000000
				// Setting gasUsed = 5000000 (50% below target)
				k.SetBlockGasWanted(ctx, 5000000) // Gas used < target
				k.SetBaseFee(ctx, math.LegacyNewDecFromBigInt(big.NewInt(1000000000)))
			},
			currentBlock:  10,
			parentBaseFee: big.NewInt(1000000000),
			checkFunc: func(s *KeeperTestSuite, result math.LegacyDec, parentBaseFee *big.Int) {
				s.T().Helper()
				parentDec := math.LegacyNewDecFromBigInt(parentBaseFee)
				require.True(s.T(), result.LT(parentDec), "Base fee should decrease when min gas price is very low")
			},
		},
		{
			name:        "small base fee delta gets clamped to minimum",
			blockMaxGas: 10000000, // parentGasTarget = 10000000 / 2 = 5000000 (ElasticityMultiplier=2)
			setupParams: func() feemarkettypes.Params {
				return feemarkettypes.Params{
					NoBaseFee:                false,
					ElasticityMultiplier:     2, // This divides MaxGas to get parentGasTarget
					BaseFeeChangeDenominator: 8,
					MinGasPrice:              math.LegacyZeroDec(),
					EnableHeight:             1,
				}
			},
			setupBlockData: func(k *keeper.Keeper, ctx sdk.Context) {
				// Set parent block gas usage slightly above target
				// parentGasTarget = blockMaxGas / ElasticityMultiplier = 10000000 / 2 = 5000000
				// Setting gasUsed = 5000001 (tiny increase above target)
				k.SetBlockGasWanted(ctx, 5000001) // Gas used > target (tiny increase)
				k.SetBaseFee(ctx, math.LegacyNewDecFromBigInt(big.NewInt(1000)))
			},
			currentBlock:  10,
			parentBaseFee: big.NewInt(1000),
			checkFunc: func(s *KeeperTestSuite, result math.LegacyDec, parentBaseFee *big.Int) {
				s.T().Helper()
				parentDec := math.LegacyNewDecFromBigInt(parentBaseFee)
				require.True(s.T(), result.GT(parentDec), "Base fee should increase even slightly due to minimum delta")
			},
		},
		{
			name:        "very high gas usage",
			blockMaxGas: 30000000, // parentGasTarget = 30000000 / 2 = 15000000 (ElasticityMultiplier=2)
			setupParams: func() feemarkettypes.Params {
				return feemarkettypes.Params{
					NoBaseFee:                false,
					ElasticityMultiplier:     2, // This divides MaxGas to get parentGasTarget
					BaseFeeChangeDenominator: 8,
					MinGasPrice:              math.LegacyZeroDec(),
					EnableHeight:             1,
				}
			},
			setupBlockData: func(k *keeper.Keeper, ctx sdk.Context) {
				// Set parent block gas usage nearly at maximum
				// parentGasTarget = blockMaxGas / ElasticityMultiplier = 30000000 / 2 = 15000000
				// Setting gasUsed = 29000000 (93% above target)
				k.SetBlockGasWanted(ctx, 29000000) // Gas used >> target (nearly full block)
				k.SetBaseFee(ctx, math.LegacyNewDecFromBigInt(big.NewInt(1000000000)))
			},
			currentBlock:  10,
			parentBaseFee: big.NewInt(1000000000),
			checkFunc: func(s *KeeperTestSuite, result math.LegacyDec, parentBaseFee *big.Int) {
				s.T().Helper()
				parentDec := math.LegacyNewDecFromBigInt(parentBaseFee)
				require.True(s.T(), result.GT(parentDec), "Base fee should increase significantly with very high gas usage")
			},
		},
		{
			name:        "very low gas usage",
			blockMaxGas: 30000000, // parentGasTarget = 30000000 / 2 = 15000000 (ElasticityMultiplier=2)
			setupParams: func() feemarkettypes.Params {
				return feemarkettypes.Params{
					NoBaseFee:                false,
					ElasticityMultiplier:     2, // This divides MaxGas to get parentGasTarget
					BaseFeeChangeDenominator: 8,
					MinGasPrice:              math.LegacyZeroDec(),
					EnableHeight:             1,
				}
			},
			setupBlockData: func(k *keeper.Keeper, ctx sdk.Context) {
				// Set parent block gas usage very low
				// parentGasTarget = blockMaxGas / ElasticityMultiplier = 30000000 / 2 = 15000000
				// Setting gasUsed = 1000000 (93% below target)
				k.SetBlockGasWanted(ctx, 1000000) // Gas used << target (very low usage)
				k.SetBaseFee(ctx, math.LegacyNewDecFromBigInt(big.NewInt(1000000000)))
			},
			currentBlock:  10,
			parentBaseFee: big.NewInt(1000000000),
			checkFunc: func(s *KeeperTestSuite, result math.LegacyDec, parentBaseFee *big.Int) {
				s.T().Helper()
				parentDec := math.LegacyNewDecFromBigInt(parentBaseFee)
				require.True(s.T(), result.LT(parentDec), "Base fee should decrease significantly with very low gas usage")
			},
		},
		{
			name:        "zero gas used",
			blockMaxGas: 30000000, // parentGasTarget = 30000000 / 2 = 15000000 (ElasticityMultiplier=2)
			setupParams: func() feemarkettypes.Params {
				return feemarkettypes.Params{
					NoBaseFee:                false,
					ElasticityMultiplier:     2, // This divides MaxGas to get parentGasTarget
					BaseFeeChangeDenominator: 8,
					MinGasPrice:              math.LegacyNewDec(50_000_000_000), // 50 minimum gas unit
					EnableHeight:             1,
				}
			},
			setupBlockData: func(k *keeper.Keeper, ctx sdk.Context) {
				// Set parent block gas usage to zero
				// parentGasTarget = blockMaxGas / ElasticityMultiplier = 30000000 / 2 = 15000000
				// Setting gasUsed = 0 (100% below target)
				k.SetBlockGasWanted(ctx, 0) // No gas used
				k.SetBaseFee(ctx, math.LegacyNewDecFromBigInt(big.NewInt(1000000000)))
			},
			currentBlock:  10,
			parentBaseFee: big.NewInt(1000000000),
			checkFunc: func(s *KeeperTestSuite, result math.LegacyDec, parentBaseFee *big.Int) {
				s.T().Helper()
				// Should be at least the minimum gas price
				factor := math.LegacyNewDecFromInt(evmtypes.GetEVMCoinDecimals().ConversionFactor())
				expectedMinGasPrice := math.LegacyNewDec(50_000_000_000).Mul(factor)
				require.True(s.T(), result.GTE(expectedMinGasPrice), "Result should be at least min gas price when no gas is used")
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			var cp *tmproto.ConsensusParams
			// reset network and context
			if tc.blockMaxGas > 0 {
				cp := integration.DefaultConsensusParams
				cp.Block.MaxGas = tc.blockMaxGas
			}
			opts := append([]network.ConfigOption{network.WithConsensusParams(cp)}, s.options...)
			nw = network.NewUnitTestNetwork(s.create, opts...)
			ctx = nw.GetContext()

			k := nw.App.GetFeeMarketKeeper()

			// Set up parameters
			params := tc.setupParams()
			err := k.SetParams(ctx, params)
			require.NoError(s.T(), err)

			// Set up block data
			tc.setupBlockData(k, ctx)

			// Set block height
			ctx = ctx.WithBlockHeight(tc.currentBlock)

			// Calculate base fee
			result := k.CalculateBaseFee(ctx)

			switch {
			case tc.expectedZero:
				s.True(result.IsNil(), "Expected zero base fee")
			case tc.checkFunc != nil:
				tc.checkFunc(s, result, tc.parentBaseFee)
			case tc.expectedResult != nil:
				expectedDec := math.LegacyNewDecFromBigInt(tc.expectedResult)
				s.Equal(expectedDec, result,
					"Expected: %s, Got: %s", expectedDec.String(), result.String())
			}
		})
	}
}
