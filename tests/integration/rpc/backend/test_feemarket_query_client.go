package backend

import (
	"github.com/stretchr/testify/mock"

	"github.com/cosmos/evm/rpc/backend/mocks"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ feemarkettypes.QueryClient = &mocks.FeeMarketQueryClient{}

// Params
func RegisterFeeMarketParams(feeMarketClient *mocks.FeeMarketQueryClient, height int64) {
	feeMarketClient.EXPECT().Params(mock.Anything, &feemarkettypes.QueryParamsRequest{}).
		Return(&feemarkettypes.QueryParamsResponse{Params: feemarkettypes.DefaultParams()}, nil)
}

func RegisterFeeMarketParamsError(feeMarketClient *mocks.FeeMarketQueryClient, height int64) {
	feeMarketClient.EXPECT().Params(mock.Anything, &feemarkettypes.QueryParamsRequest{}).
		Return(nil, sdkerrors.ErrInvalidRequest)
}
