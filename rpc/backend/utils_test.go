package backend_test

import (
	"math/big"
	"testing"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/rpc/backend"
	"github.com/cosmos/evm/testutil/constants"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"
)

func TestCalcBaseFee(t *testing.T) {
	evmConfigurator := evmtypes.NewEVMConfigurator().
		WithEVMCoinInfo(constants.ExampleChainCoinInfo[constants.ExampleChainID])
	err := evmConfigurator.Configure()
	require.NoError(t, err)
	testCases := []struct {
		name           string
		config         *params.ChainConfig
		parent         *ethtypes.Header
		params         feemarkettypes.Params
		expectedResult *big.Int
		expectedError  string
	}{
		{
			name: "pre-London block - returns InitialBaseFee",
			config: &params.ChainConfig{
				LondonBlock: big.NewInt(100), // London activated at block 100
			},
			parent: &ethtypes.Header{
				Number:  big.NewInt(50), // Block 50 (pre-London)
				BaseFee: big.NewInt(1000000000),
			},
			params: feemarkettypes.Params{
				ElasticityMultiplier:     2,
				BaseFeeChangeDenominator: 8,
				MinGasPrice:              sdkmath.LegacyZeroDec(),
			},
			expectedResult: big.NewInt(params.InitialBaseFee), // 1000000000
			expectedError:  "",
		},
		{
			name: "ElasticityMultiplier is zero - returns error",
			config: &params.ChainConfig{
				LondonBlock: big.NewInt(0), // London activated from genesis
			},
			parent: &ethtypes.Header{
				Number:   big.NewInt(10),
				BaseFee:  big.NewInt(1000000000),
				GasLimit: 10000000,
				GasUsed:  5000000,
			},
			params: feemarkettypes.Params{
				ElasticityMultiplier:     0, // Invalid - zero
				BaseFeeChangeDenominator: 8,
				MinGasPrice:              sdkmath.LegacyZeroDec(),
			},
			expectedResult: nil,
			expectedError:  "ElasticityMultiplier cannot be 0 as it's checked in the params validation",
		},
		{
			name: "gas used equals target - base fee unchanged",
			config: &params.ChainConfig{
				LondonBlock: big.NewInt(0),
			},
			parent: &ethtypes.Header{
				Number:   big.NewInt(10),
				BaseFee:  big.NewInt(1000000000),
				GasLimit: 10000000,
				GasUsed:  5000000, // Target = 10000000 / 2 = 5000000
			},
			params: feemarkettypes.Params{
				ElasticityMultiplier:     2,
				BaseFeeChangeDenominator: 8,
				MinGasPrice:              sdkmath.LegacyZeroDec(),
			},
			expectedResult: big.NewInt(1000000000), // Unchanged
			expectedError:  "",
		},
		{
			name: "gas used > target - base fee increases",
			config: &params.ChainConfig{
				LondonBlock: big.NewInt(0),
			},
			parent: &ethtypes.Header{
				Number:   big.NewInt(10),
				BaseFee:  big.NewInt(1000000000),
				GasLimit: 10000000,
				GasUsed:  7500000, // Target = 5000000, used > target
			},
			params: feemarkettypes.Params{
				ElasticityMultiplier:     2,
				BaseFeeChangeDenominator: 8,
				MinGasPrice:              sdkmath.LegacyZeroDec(),
			},
			expectedResult: func() *big.Int {
				// gasUsedDelta = 7500000 - 5000000 = 2500000
				// baseFeeDelta = max(1, 1000000000 * 2500000 / 5000000 / 8)
				// baseFeeDelta = max(1, 62500000)
				// result = 1000000000 + 62500000 = 1062500000
				return big.NewInt(1062500000)
			}(),
			expectedError: "",
		},
		{
			name: "base fee decrease with low min gas price",
			config: &params.ChainConfig{
				LondonBlock: big.NewInt(0),
			},
			parent: &ethtypes.Header{
				Number:   big.NewInt(10),
				BaseFee:  big.NewInt(1000000000),
				GasLimit: 10000000,
				GasUsed:  2500000,
			},
			params: feemarkettypes.Params{
				ElasticityMultiplier:     2,
				BaseFeeChangeDenominator: 8,
				MinGasPrice:              sdkmath.LegacyNewDecWithPrec(1, 12), // Very low
			},
			expectedResult: func() *big.Int {
				// result = 1000000000 - 62500000 = 937500000
				// minGasPrice is very low, so doesn't affect result
				return big.NewInt(937500000)
			}(),
			expectedError: "",
		},
		{
			name: "small base fee delta gets clamped to 1",
			config: &params.ChainConfig{
				LondonBlock: big.NewInt(0),
			},
			parent: &ethtypes.Header{
				Number:   big.NewInt(10),
				BaseFee:  big.NewInt(1000),
				GasLimit: 10000000,
				GasUsed:  5000001, // Tiny increase
			},
			params: feemarkettypes.Params{
				ElasticityMultiplier:     2,
				BaseFeeChangeDenominator: 8,
				MinGasPrice:              sdkmath.LegacyZeroDec(),
			},
			expectedResult: func() *big.Int {
				// gasUsedDelta = 1
				// baseFeeDelta = max(1, 1000 * 1 / 5000000 / 8) = max(1, 0) = 1
				// result = 1000 + 1 = 1001
				return big.NewInt(1001)
			}(),
			expectedError: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := backend.CalcBaseFee(tc.config, tc.parent, tc.params)

			if tc.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tc.expectedResult, result,
					"Expected: %s, Got: %s", tc.expectedResult.String(), result.String())
			}
		})
	}
}
