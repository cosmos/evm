package keeper_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/vm/keeper"
	vmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/evm/x/vm/types/mocks"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func TestDeductFees(t *testing.T) {
	testCases := []struct {
		name          string
		feeCoin       sdk.Coin
		sdkDenom      banktypes.DenomUnit
		evmCoinInfo   vmtypes.EvmCoinInfo
		expectedPanic bool
		expectedErr   error
	}{
		{
			name:          "happy path coins matching",
			feeCoin:       sdk.NewCoin("feecoin", sdkmath.NewInt(100)),
			sdkDenom:      banktypes.DenomUnit{Denom: "feecoin", Exponent: 18},
			evmCoinInfo:   vmtypes.EvmCoinInfo{Denom: "feecoin", Decimals: 18},
			expectedPanic: false,
			expectedErr:   nil,
		},
		{
			name:          "panic on decimal mismatch",
			feeCoin:       sdk.NewCoin("feecoin", sdkmath.NewInt(100)),
			sdkDenom:      banktypes.DenomUnit{Denom: "feecoin", Exponent: 6},
			evmCoinInfo:   vmtypes.EvmCoinInfo{Denom: "feecoin", Decimals: 18},
			expectedPanic: true,
			expectedErr:   nil,
		},
		{
			name:          "error on bank call failure",
			feeCoin:       sdk.NewCoin("feecoin", sdkmath.NewInt(100)),
			sdkDenom:      banktypes.DenomUnit{Denom: "feecoin", Exponent: 18},
			evmCoinInfo:   vmtypes.EvmCoinInfo{Denom: "feecoin", Decimals: 18},
			expectedPanic: false,
			expectedErr:   errors.New("foo"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			vmKeeper := mocks.NewVMKeeper(t)
			bankKeeper := mocks.NewBankKeeper(t)

			key1 := storetypes.NewKVStoreKey(vmtypes.StoreKey)
			key2 := storetypes.NewTransientStoreKey("foobar")

			ctx := testutil.DefaultContext(key1, key2)
			feeCoins := sdk.NewCoins(tc.feeCoin)
			acc := authtypes.BaseAccount{Address: "foobar"}

			vmKeeper.On("GetEvmCoinInfo", mock.Anything).Return(tc.evmCoinInfo)
			bankKeeper.On("GetDenomMetaData", mock.Anything, mock.Anything).Return(
				banktypes.Metadata{DenomUnits: []*banktypes.DenomUnit{&tc.sdkDenom}}, true)
			if !tc.expectedPanic {
				bankKeeper.On("SendCoinsFromAccountToModuleVirtual", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(tc.expectedErr)
			}

			// Run DeductFees
			var err error
			if tc.expectedPanic {
				require.Panics(t, func() { err = keeper.DeductFees(bankKeeper, vmKeeper, ctx, &acc, feeCoins) })
			} else {
				err = keeper.DeductFees(bankKeeper, vmKeeper, ctx, &acc, feeCoins)
			}

			// Validate Results
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
