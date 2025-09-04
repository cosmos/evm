package wrappers_test

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	testconfig "github.com/cosmos/evm/testutil/config"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/evm/x/vm/wrappers"
	"github.com/cosmos/evm/x/vm/wrappers/testutil"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestGetBaseFee(t *testing.T) {
	defaultCoinInfo := testconfig.DefaultChainConfig.CoinInfo
	sixDecimalsCoinInfo := testconfig.SixDecimalsChainConfig.CoinInfo

	testCases := []struct {
		name      string
		coinInfo  evmtypes.EvmCoinInfo
		expResult *big.Int
		mockSetup func(*testutil.MockFeeMarketKeeper)
	}{
		{
			name:      "success - does not convert 18 decimals",
			coinInfo:  defaultCoinInfo,
			expResult: big.NewInt(1e18), // 1 token in 18 decimals
			mockSetup: func(mfk *testutil.MockFeeMarketKeeper) {
				mfk.EXPECT().
					GetBaseFee(gomock.Any()).
					Return(sdkmath.LegacyNewDec(1e18))
			},
		},
		{
			name:      "success - convert 6 decimals to 18 decimals",
			coinInfo:  sixDecimalsCoinInfo,
			expResult: big.NewInt(1e18), // 1 token in 18 decimals
			mockSetup: func(mfk *testutil.MockFeeMarketKeeper) {
				mfk.EXPECT().
					GetBaseFee(gomock.Any()).
					Return(sdkmath.LegacyNewDec(1_000_000))
			},
		},
		{
			name:      "success - nil base fee",
			coinInfo:  sixDecimalsCoinInfo,
			expResult: nil,
			mockSetup: func(mfk *testutil.MockFeeMarketKeeper) {
				mfk.EXPECT().
					GetBaseFee(gomock.Any()).
					Return(sdkmath.LegacyDec{})
			},
		},
		{
			name:      "success - small amount 18 decimals",
			coinInfo:  sixDecimalsCoinInfo,
			expResult: big.NewInt(1e12), // 0.000001 token in 18 decimals
			mockSetup: func(mfk *testutil.MockFeeMarketKeeper) {
				mfk.EXPECT().
					GetBaseFee(gomock.Any()).
					Return(sdkmath.LegacyNewDec(1))
			},
		},
		{
			name:      "success - base fee is zero",
			coinInfo:  sixDecimalsCoinInfo,
			expResult: big.NewInt(0),
			mockSetup: func(mfk *testutil.MockFeeMarketKeeper) {
				mfk.EXPECT().
					GetBaseFee(gomock.Any()).
					Return(sdkmath.LegacyNewDec(0))
			},
		},
		{
			name:      "success - truncate decimals with number less than 1",
			coinInfo:  sixDecimalsCoinInfo,
			expResult: big.NewInt(0), // 0.000001 token in 18 decimals
			mockSetup: func(mfk *testutil.MockFeeMarketKeeper) {
				mfk.EXPECT().
					GetBaseFee(gomock.Any()).
					Return(sdkmath.LegacyNewDecWithPrec(1, 13)) // multiplied by 1e12 is still less than 1
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup EVM configurator to have access to the EVM coin info.
			configurator := evmtypes.NewEVMConfigurator()
			configurator.ResetTestConfig()
			err := configurator.WithEVMCoinInfo(tc.coinInfo).Configure()
			require.NoError(t, err, "failed to configure EVMConfigurator")

			ctrl := gomock.NewController(t)
			mockFeeMarketKeeper := testutil.NewMockFeeMarketKeeper(ctrl)
			tc.mockSetup(mockFeeMarketKeeper)

			feeMarketWrapper := wrappers.NewFeeMarketWrapper(mockFeeMarketKeeper)
			result := feeMarketWrapper.GetBaseFee(sdk.Context{})

			require.Equal(t, tc.expResult, result)
		})
	}
}

func TestCalculateBaseFee(t *testing.T) {
	defaultCoinInfo := testconfig.DefaultChainConfig.CoinInfo
	sixDecimalsCoinInfo := testconfig.SixDecimalsChainConfig.CoinInfo

	testCases := []struct {
		name      string
		coinInfo  evmtypes.EvmCoinInfo
		baseFee   sdkmath.LegacyDec
		expResult *big.Int
		mockSetup func(*testutil.MockFeeMarketKeeper)
	}{
		{
			name:      "success - does not convert 18 decimals",
			coinInfo:  defaultCoinInfo,
			expResult: big.NewInt(1e18), // 1 token in 18 decimals
			mockSetup: func(mfk *testutil.MockFeeMarketKeeper) {
				mfk.EXPECT().
					CalculateBaseFee(gomock.Any()).
					Return(sdkmath.LegacyNewDec(1e18))
			},
		},
		{
			name:      "success - convert 6 decimals to 18 decimals",
			coinInfo:  sixDecimalsCoinInfo,
			expResult: big.NewInt(1e18), // 1 token in 18 decimals
			mockSetup: func(mfk *testutil.MockFeeMarketKeeper) {
				mfk.EXPECT().
					CalculateBaseFee(gomock.Any()).
					Return(sdkmath.LegacyNewDec(1_000_000))
			},
		},
		{
			name:      "success - nil base fee",
			coinInfo:  sixDecimalsCoinInfo,
			expResult: nil,
			mockSetup: func(mfk *testutil.MockFeeMarketKeeper) {
				mfk.EXPECT().
					CalculateBaseFee(gomock.Any()).
					Return(sdkmath.LegacyDec{})
			},
		},
		{
			name:      "success - small amount 18 decimals",
			coinInfo:  sixDecimalsCoinInfo,
			expResult: big.NewInt(1e12), // 0.000001 token in 18 decimals
			mockSetup: func(mfk *testutil.MockFeeMarketKeeper) {
				mfk.EXPECT().
					CalculateBaseFee(gomock.Any()).
					Return(sdkmath.LegacyNewDec(1))
			},
		},
		{
			name:      "success - base fee is zero",
			coinInfo:  sixDecimalsCoinInfo,
			expResult: big.NewInt(0),
			mockSetup: func(mfk *testutil.MockFeeMarketKeeper) {
				mfk.EXPECT().
					CalculateBaseFee(gomock.Any()).
					Return(sdkmath.LegacyNewDec(0))
			},
		},
		{
			name:      "success - truncate decimals with number less than 1",
			coinInfo:  sixDecimalsCoinInfo,
			expResult: big.NewInt(0), // 0.000001 token in 18 decimals
			mockSetup: func(mfk *testutil.MockFeeMarketKeeper) {
				mfk.EXPECT().
					CalculateBaseFee(gomock.Any()).
					Return(sdkmath.LegacyNewDecWithPrec(1, 13)) // multiplied by 1e12 is still less than 1
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup EVM configurator to have access to the EVM coin info.
			configurator := evmtypes.NewEVMConfigurator()
			configurator.ResetTestConfig()
			err := configurator.WithEVMCoinInfo(tc.coinInfo).Configure()
			require.NoError(t, err, "failed to configure EVMConfigurator")

			ctrl := gomock.NewController(t)
			mockFeeMarketKeeper := testutil.NewMockFeeMarketKeeper(ctrl)
			tc.mockSetup(mockFeeMarketKeeper)

			feeMarketWrapper := wrappers.NewFeeMarketWrapper(mockFeeMarketKeeper)
			result := feeMarketWrapper.CalculateBaseFee(sdk.Context{})

			require.Equal(t, tc.expResult, result)
		})
	}
}

func TestGetParams(t *testing.T) {
	defaultCoinInfo := testconfig.DefaultChainConfig.CoinInfo
	sixDecimalsCoinInfo := testconfig.SixDecimalsChainConfig.CoinInfo

	testCases := []struct {
		name      string
		coinInfo  evmtypes.EvmCoinInfo
		expParams feemarkettypes.Params
		mockSetup func(*testutil.MockFeeMarketKeeper)
	}{
		{
			name:     "success - convert 6 decimals to 18 decimals",
			coinInfo: sixDecimalsCoinInfo,
			expParams: feemarkettypes.Params{
				BaseFee:     sdkmath.LegacyNewDec(1e18),
				MinGasPrice: sdkmath.LegacyNewDec(1e18),
			},
			mockSetup: func(mfk *testutil.MockFeeMarketKeeper) {
				mfk.EXPECT().
					GetParams(gomock.Any()).
					Return(feemarkettypes.Params{
						BaseFee:     sdkmath.LegacyNewDec(1_000_000),
						MinGasPrice: sdkmath.LegacyNewDec(1_000_000),
					})
			},
		},
		{
			name:     "success - does not convert 18 decimals",
			coinInfo: defaultCoinInfo,
			expParams: feemarkettypes.Params{
				BaseFee:     sdkmath.LegacyNewDec(1e18),
				MinGasPrice: sdkmath.LegacyNewDec(1e18),
			},
			mockSetup: func(mfk *testutil.MockFeeMarketKeeper) {
				mfk.EXPECT().
					GetParams(gomock.Any()).
					Return(feemarkettypes.Params{
						BaseFee:     sdkmath.LegacyNewDec(1e18),
						MinGasPrice: sdkmath.LegacyNewDec(1e18),
					})
			},
		},
		{
			name:     "success - nil base fee",
			coinInfo: defaultCoinInfo,
			expParams: feemarkettypes.Params{
				MinGasPrice: sdkmath.LegacyNewDec(1e18),
			},
			mockSetup: func(mfk *testutil.MockFeeMarketKeeper) {
				mfk.EXPECT().
					GetParams(gomock.Any()).
					Return(feemarkettypes.Params{
						MinGasPrice: sdkmath.LegacyNewDec(1e18),
					})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup EVM configurator to have access to the EVM coin info.
			configurator := evmtypes.NewEVMConfigurator()
			configurator.ResetTestConfig()
			err := configurator.WithEVMCoinInfo(tc.coinInfo).Configure()
			require.NoError(t, err, "failed to configure EVMConfigurator")

			ctrl := gomock.NewController(t)
			mockFeeMarketKeeper := testutil.NewMockFeeMarketKeeper(ctrl)
			tc.mockSetup(mockFeeMarketKeeper)

			feeMarketWrapper := wrappers.NewFeeMarketWrapper(mockFeeMarketKeeper)
			result := feeMarketWrapper.GetParams(sdk.Context{})

			require.Equal(t, tc.expParams, result)
		})
	}
}
