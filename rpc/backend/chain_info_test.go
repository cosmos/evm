package backend

import (
	"context"
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/rpc/backend/mocks"
	rpctypes "github.com/cosmos/evm/rpc/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
)

func TestSuggestGasTipCap_LargeBaseFee(t *testing.T) {
	backend := setupMockBackend(t)
	mockFeeMarketClient := backend.QueryClient.FeeMarket.(*mocks.FeeMarketQueryClient)

	// Use a baseFee that exceeds MaxInt64 — the old int64 arithmetic
	// would silently overflow and produce incorrect results.
	largeBaseFee := new(big.Int).SetUint64(math.MaxUint64)

	defaultParams := feemarkettypes.DefaultParams()
	mockFeeMarketClient.On("Params", mock.Anything, mock.Anything, mock.Anything).
		Return(&feemarkettypes.QueryParamsResponse{Params: defaultParams}, nil)

	tip, err := backend.SuggestGasTipCap(rpctypes.NewContextWithHeight(1), largeBaseFee)
	require.NoError(t, err)
	require.NotNil(t, tip)
	require.True(t, tip.Sign() >= 0, "gas tip should be non-negative")

	// With big.Int arithmetic, the result should be:
	// maxDelta = baseFee * (elasticity - 1) / denom
	elasticity := new(big.Int).SetUint64(uint64(defaultParams.ElasticityMultiplier))
	denom := new(big.Int).SetUint64(uint64(defaultParams.BaseFeeChangeDenominator))
	expected := new(big.Int).Sub(elasticity, big.NewInt(1))
	expected.Mul(expected, largeBaseFee)
	expected.Div(expected, denom)
	require.Equal(t, expected, tip)
}

func TestSuggestGasTipCap_NilBaseFee(t *testing.T) {
	backend := setupMockBackend(t)

	tip, err := backend.SuggestGasTipCap(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(0), tip)
}

func TestSuggestGasTipCap_NormalBaseFee(t *testing.T) {
	backend := setupMockBackend(t)
	mockFeeMarketClient := backend.QueryClient.FeeMarket.(*mocks.FeeMarketQueryClient)

	baseFee := big.NewInt(1000000000) // 1 gwei

	defaultParams := feemarkettypes.DefaultParams()
	mockFeeMarketClient.On("Params", mock.Anything, mock.Anything, mock.Anything).
		Return(&feemarkettypes.QueryParamsResponse{Params: defaultParams}, nil)

	tip, err := backend.SuggestGasTipCap(rpctypes.NewContextWithHeight(1), baseFee)
	require.NoError(t, err)
	require.NotNil(t, tip)
	require.True(t, tip.Sign() >= 0)
}
