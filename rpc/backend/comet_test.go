package backend

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	tmrpctypes "github.com/cometbft/cometbft/rpc/core/types"

	"github.com/cosmos/evm/rpc/backend/mocks"
)

func TestCometBlockResultByNumber_NilHeightErrorPath(t *testing.T) {
	testCases := []struct {
		name      string
		height    *int64
		rpcErr    error
		expectErr bool
	}{
		{
			name:      "nil height, BlockResults succeeds",
			height:    nil,
			rpcErr:    nil,
			expectErr: false,
		},
		{
			name:      "non-zero height, BlockResults succeeds",
			height:    func() *int64 { h := int64(5); return &h }(),
			rpcErr:    nil,
			expectErr: false,
		},
		{
			name:      "zero height remapped to nil, BlockResults errors - must not panic",
			height:    func() *int64 { h := int64(0); return &h }(),
			rpcErr:    errors.New("connection timeout"),
			expectErr: true,
		},
		{
			name:      "nil height, BlockResults errors",
			height:    nil,
			rpcErr:    errors.New("pruned state"),
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backend := setupMockBackend(t)
			mockClient := backend.ClientCtx.Client.(*mocks.Client)

			// The function remaps height=0 to nil before calling BlockResults,
			// so always mock with nil when input is 0.
			var mockHeight *int64
			if tc.height != nil && *tc.height != 0 {
				mockHeight = tc.height
			}

			if tc.rpcErr != nil {
				mockClient.On("BlockResults", mock.Anything, mockHeight).
					Return((*tmrpctypes.ResultBlockResults)(nil), tc.rpcErr).Once()
			} else {
				mockClient.On("BlockResults", mock.Anything, mockHeight).
					Return(&tmrpctypes.ResultBlockResults{Height: 1}, nil).Once()
			}

			require.NotPanics(t, func() {
				_, err := backend.CometBlockResultByNumber(context.Background(), tc.height)
				if tc.expectErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		})
	}
}
