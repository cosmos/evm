package wrappers_test

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	testconstants "github.com/cosmos/evm/testutil/constants"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/evm/x/vm/wrappers"
	"github.com/cosmos/evm/x/vm/wrappers/testutil"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// --------------------------------------TRANSACTIONS-----------------------------------------------

func TestMintAmountToAccount(t *testing.T) {
	eighteenDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]
	sixDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.SixDecimalsChainID]

	testCases := []struct {
		name      string
		coinInfo  evmtypes.EvmCoinInfo
		evmDenom  string
		amount    *big.Int
		recipient sdk.AccAddress
		expectErr string
		mockSetup func(*testutil.MockBankWrapper)
	}{
		{
			name:      "success - convert evm coin denom to extended denom",
			coinInfo:  sixDecimalsCoinInfo,
			evmDenom:  sixDecimalsCoinInfo.Denom,
			amount:    big.NewInt(1e18), // 1 token in 18 decimals
			recipient: sdk.AccAddress([]byte("test_address")),
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoin := sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e18)) // 1 token in 18 decimals
				expectedCoins := sdk.NewCoins(expectedCoin)

				mbk.EXPECT().
					MintCoins(gomock.Any(), evmtypes.ModuleName, expectedCoins).
					Return(nil)

				mbk.EXPECT().
					SendCoinsFromModuleToAccount(
						gomock.Any(),
						evmtypes.ModuleName,
						sdk.AccAddress([]byte("test_address")),
						expectedCoins,
					).Return(nil)
			},
		},
		{
			name:      "success - 18 decimals amount not modified",
			coinInfo:  eighteenDecimalsCoinInfo,
			evmDenom:  eighteenDecimalsCoinInfo.Denom,
			amount:    big.NewInt(1e18), // 1 token in 18 decimals
			recipient: sdk.AccAddress([]byte("test_address")),
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoin := sdk.NewCoin(eighteenDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18))
				expectedCoins := sdk.NewCoins(expectedCoin)

				mbk.EXPECT().
					MintCoins(gomock.Any(), evmtypes.ModuleName, expectedCoins).
					Return(nil)

				mbk.EXPECT().
					SendCoinsFromModuleToAccount(
						gomock.Any(),
						evmtypes.ModuleName,
						sdk.AccAddress([]byte("test_address")),
						expectedCoins,
					).Return(nil)
			},
		},
		{
			name:      "fail - mint coins error",
			coinInfo:  sixDecimalsCoinInfo,
			evmDenom:  sixDecimalsCoinInfo.Denom,
			amount:    big.NewInt(1e18),
			recipient: sdk.AccAddress([]byte("test_address")),
			expectErr: "failed to mint coins to account in bank wrapper",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoin := sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e18))
				expectedCoins := sdk.NewCoins(expectedCoin)

				mbk.EXPECT().
					MintCoins(gomock.Any(), evmtypes.ModuleName, expectedCoins).
					Return(errors.New("mint error"))
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

			// Setup mock controller
			ctrl := gomock.NewController(t)

			mockBankKeeper := testutil.NewMockBankWrapper(ctrl)
			tc.mockSetup(mockBankKeeper)

			bankWrapper := wrappers.NewBankWrapper(mockBankKeeper)
			err = bankWrapper.MintAmountToAccount(context.Background(), tc.recipient, tc.amount)

			if tc.expectErr != "" {
				require.ErrorContains(t, err, tc.expectErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBurnAmountFromAccount(t *testing.T) {
	eighteenDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]
	sixDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.SixDecimalsChainID]

	account := sdk.AccAddress([]byte("test_address"))

	testCases := []struct {
		name      string
		coinInfo  evmtypes.EvmCoinInfo
		amount    *big.Int
		expectErr string
		mockSetup func(*testutil.MockBankWrapper)
	}{
		{
			name:      "success - convert evm coin denom to extended denom",
			coinInfo:  sixDecimalsCoinInfo,
			amount:    big.NewInt(1e18),
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoin := sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e18))
				expectedCoins := sdk.NewCoins(expectedCoin)

				mbk.EXPECT().
					SendCoinsFromAccountToModule(
						gomock.Any(),
						account,
						evmtypes.ModuleName,
						expectedCoins,
					).Return(nil)

				mbk.EXPECT().
					BurnCoins(gomock.Any(), evmtypes.ModuleName, expectedCoins).
					Return(nil)
			},
		},
		{
			name:      "success - 18 decimals amount not modified",
			coinInfo:  eighteenDecimalsCoinInfo,
			amount:    big.NewInt(1e18),
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoin := sdk.NewCoin(eighteenDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18))
				expectedCoins := sdk.NewCoins(expectedCoin)

				mbk.EXPECT().
					SendCoinsFromAccountToModule(
						gomock.Any(),
						account,
						evmtypes.ModuleName,
						expectedCoins,
					).Return(nil)

				mbk.EXPECT().
					BurnCoins(gomock.Any(), evmtypes.ModuleName, expectedCoins).
					Return(nil)
			},
		},
		{
			name:      "fail - send coins error",
			coinInfo:  sixDecimalsCoinInfo,
			amount:    big.NewInt(1e18),
			expectErr: "failed to burn coins from account in bank wrapper",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoin := sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e18))
				expectedCoins := sdk.NewCoins(expectedCoin)

				mbk.EXPECT().
					SendCoinsFromAccountToModule(
						gomock.Any(),
						account,
						evmtypes.ModuleName,
						expectedCoins,
					).Return(errors.New("send error"))
			},
		},
		{
			name:      "fail - send burn error",
			coinInfo:  sixDecimalsCoinInfo,
			amount:    big.NewInt(1e18),
			expectErr: "burn error",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoin := sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e18))
				expectedCoins := sdk.NewCoins(expectedCoin)

				mbk.EXPECT().
					SendCoinsFromAccountToModule(
						gomock.Any(),
						account,
						evmtypes.ModuleName,
						expectedCoins,
					).Return(errors.New("burn error"))
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

			// Setup mock controller
			ctrl := gomock.NewController(t)

			mockBankKeeper := testutil.NewMockBankWrapper(ctrl)
			tc.mockSetup(mockBankKeeper)

			bankWrapper := wrappers.NewBankWrapper(mockBankKeeper)
			err = bankWrapper.BurnAmountFromAccount(context.Background(), account, tc.amount)

			if tc.expectErr != "" {
				require.ErrorContains(t, err, tc.expectErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSendCoinsFromModuleToAccount(t *testing.T) {
	eighteenDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]
	sixDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.SixDecimalsChainID]

	account := sdk.AccAddress([]byte("test_address"))

	testCases := []struct {
		name      string
		coinInfo  evmtypes.EvmCoinInfo
		coins     func() sdk.Coins
		expectErr string
		mockSetup func(*testutil.MockBankWrapper)
	}{
		{
			name:     "success - does not convert 18 decimals amount with single token",
			coinInfo: eighteenDecimalsCoinInfo,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(eighteenDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(eighteenDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18)),
				}...)

				mbk.EXPECT().
					SendCoinsFromModuleToAccount(
						gomock.Any(),
						evmtypes.ModuleName,
						account,
						expectedCoins,
					).Return(nil)
			},
		},
		{
			name:     "success - convert evm coin denom to extended denom with single token",
			coinInfo: sixDecimalsCoinInfo,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(sixDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e18)),
				}...)

				mbk.EXPECT().
					SendCoinsFromModuleToAccount(
						gomock.Any(),
						evmtypes.ModuleName,
						account,
						expectedCoins,
					).Return(nil)
			},
		},
		{
			name:     "success - does not convert 18 decimals amount with multiple tokens",
			coinInfo: eighteenDecimalsCoinInfo,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(eighteenDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18)),
					sdk.NewCoin("something", sdkmath.NewInt(3e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(eighteenDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18)),
					sdk.NewCoin("something", sdkmath.NewInt(3e18)),
				}...)

				mbk.EXPECT().
					SendCoinsFromModuleToAccount(
						gomock.Any(),
						evmtypes.ModuleName,
						account,
						expectedCoins,
					).Return(nil)
			},
		},
		{
			name:     "success - convert evm coin denom to extended denom with multiple tokens",
			coinInfo: sixDecimalsCoinInfo,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(sixDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18)),
					sdk.NewCoin("something", sdkmath.NewInt(3e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e18)),
					sdk.NewCoin("something", sdkmath.NewInt(3e18)),
				}...)

				mbk.EXPECT().
					SendCoinsFromModuleToAccount(
						gomock.Any(),
						evmtypes.ModuleName,
						account,
						expectedCoins,
					).Return(nil)
			},
		},
		{
			name:     "success - no op if coin is zero",
			coinInfo: sixDecimalsCoinInfo,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(sixDecimalsCoinInfo.Denom, sdkmath.ZeroInt()),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				mbk.EXPECT().
					SendCoinsFromModuleToAccount(
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
					).Times(0)
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

			// Setup mock controller
			ctrl := gomock.NewController(t)

			mockBankKeeper := testutil.NewMockBankWrapper(ctrl)
			tc.mockSetup(mockBankKeeper)

			bankWrapper := wrappers.NewBankWrapper(mockBankKeeper)
			err = bankWrapper.SendCoinsFromModuleToAccount(context.Background(), evmtypes.ModuleName, account, tc.coins())

			if tc.expectErr != "" {
				require.ErrorContains(t, err, tc.expectErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSendCoinsFromAccountToModule(t *testing.T) {
	eighteenDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]
	sixDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.SixDecimalsChainID]

	account := sdk.AccAddress([]byte("test_address"))

	testCases := []struct {
		name      string
		coinInfo  evmtypes.EvmCoinInfo
		coins     func() sdk.Coins
		expectErr string
		mockSetup func(*testutil.MockBankWrapper)
	}{
		{
			name:     "success - does not convert 18 decimals amount with single token",
			coinInfo: eighteenDecimalsCoinInfo,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(eighteenDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(eighteenDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18)),
				}...)

				mbk.EXPECT().
					SendCoinsFromAccountToModule(
						gomock.Any(),
						account,
						evmtypes.ModuleName,
						expectedCoins,
					).Return(nil)
			},
		},
		{
			name:     "success - convert evm coin denom to extended denom with single token",
			coinInfo: sixDecimalsCoinInfo,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(sixDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e18)),
				}...)

				mbk.EXPECT().
					SendCoinsFromAccountToModule(
						gomock.Any(),
						account,
						evmtypes.ModuleName,
						expectedCoins,
					).Return(nil)
			},
		},
		{
			name:     "success - does not convert 18 decimals amount with multiple tokens",
			coinInfo: eighteenDecimalsCoinInfo,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(eighteenDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18)),
					sdk.NewCoin("something", sdkmath.NewInt(3e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(eighteenDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18)),
					sdk.NewCoin("something", sdkmath.NewInt(3e18)),
				}...)

				mbk.EXPECT().
					SendCoinsFromAccountToModule(
						gomock.Any(),
						account,
						evmtypes.ModuleName,
						expectedCoins,
					).Return(nil)
			},
		},
		{
			name:     "success - convert evm coin denom to extended denom with multiple tokens",
			coinInfo: sixDecimalsCoinInfo,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(sixDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18)),
					sdk.NewCoin("something", sdkmath.NewInt(3e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e18)),
					sdk.NewCoin("something", sdkmath.NewInt(3e18)),
				}...)

				mbk.EXPECT().
					SendCoinsFromAccountToModule(
						gomock.Any(),
						account,
						evmtypes.ModuleName,
						expectedCoins,
					).Return(nil)
			},
		},
		{
			name:     "success - no op if coin is zero",
			coinInfo: sixDecimalsCoinInfo,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(sixDecimalsCoinInfo.Denom, sdkmath.ZeroInt()),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				mbk.EXPECT().
					SendCoinsFromAccountToModule(
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
					).Times(0)
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

			// Setup mock controller
			ctrl := gomock.NewController(t)

			mockBankKeeper := testutil.NewMockBankWrapper(ctrl)
			tc.mockSetup(mockBankKeeper)

			bankWrapper := wrappers.NewBankWrapper(mockBankKeeper)
			err = bankWrapper.SendCoinsFromAccountToModule(context.Background(), account, evmtypes.ModuleName, tc.coins())

			if tc.expectErr != "" {
				require.ErrorContains(t, err, tc.expectErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ----------------------------------------QUERIES-------------------------------------------------

func TestGetBalance(t *testing.T) {
	eighteenDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]
	sixDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.SixDecimalsChainID]

	maxInt64 := int64(9223372036854775807)
	account := sdk.AccAddress([]byte("test_address"))

	testCases := []struct {
		name      string
		coinInfo  evmtypes.EvmCoinInfo
		evmDenom  string
		expCoin   sdk.Coin
		expErr    string
		expPanic  string
		mockSetup func(*testutil.MockBankWrapper)
	}{
		{
			name:     "success - convert 6 decimals amount to 18 decimals",
			coinInfo: sixDecimalsCoinInfo,
			evmDenom: sixDecimalsCoinInfo.Denom,
			expCoin:  sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e18)),
			expErr:   "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e18))

				mbk.EXPECT().
					GetBalance(
						gomock.Any(),
						account,
						sixDecimalsCoinInfo.ExtendedDenom,
					).Return(returnedCoin)
			},
		},
		{
			name:     "success - convert max int 6 decimals amount to 18 decimals",
			coinInfo: sixDecimalsCoinInfo,
			evmDenom: sixDecimalsCoinInfo.Denom,
			expCoin:  sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e12).MulRaw(maxInt64)),
			expErr:   "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e12).MulRaw(maxInt64))

				mbk.EXPECT().
					GetBalance(
						gomock.Any(),
						account,
						sixDecimalsCoinInfo.ExtendedDenom,
					).Return(returnedCoin)
			},
		},
		{
			name:     "success - does not convert 18 decimals amount",
			coinInfo: eighteenDecimalsCoinInfo,
			evmDenom: eighteenDecimalsCoinInfo.Denom,
			expCoin:  sdk.NewCoin(eighteenDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18)),
			expErr:   "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin(eighteenDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18))

				mbk.EXPECT().
					GetBalance(
						gomock.Any(),
						account,
						eighteenDecimalsCoinInfo.Denom,
					).Return(returnedCoin)
			},
		},
		{
			name:     "success - zero balance",
			coinInfo: sixDecimalsCoinInfo,
			evmDenom: sixDecimalsCoinInfo.Denom,
			expCoin:  sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(0)),
			expErr:   "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(0))

				mbk.EXPECT().
					GetBalance(
						gomock.Any(),
						account,
						sixDecimalsCoinInfo.ExtendedDenom,
					).Return(returnedCoin)
			},
		},
		{
			name:     "success - small amount (less than 1 full token)",
			coinInfo: sixDecimalsCoinInfo,
			evmDenom: sixDecimalsCoinInfo.Denom,
			expCoin:  sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e14)), // 0.0001 token in 18 decimals
			expErr:   "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e14)) // 0.0001 token in 6 decimals

				mbk.EXPECT().
					GetBalance(
						gomock.Any(),
						account,
						sixDecimalsCoinInfo.ExtendedDenom,
					).Return(returnedCoin)
			},
		},
		{
			name:      "panic - wrong evm denom",
			coinInfo:  eighteenDecimalsCoinInfo,
			evmDenom:  "wrong_denom",
			expPanic:  "expected evm denom",
			mockSetup: func(mbk *testutil.MockBankWrapper) {},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup EVM configurator to have access to the EVM coin info.
			configurator := evmtypes.NewEVMConfigurator()
			configurator.ResetTestConfig()
			err := configurator.WithEVMCoinInfo(tc.coinInfo).Configure()
			require.NoError(t, err, "failed to configure EVMConfigurator")

			// Setup mock controller
			ctrl := gomock.NewController(t)

			mockBankKeeper := testutil.NewMockBankWrapper(ctrl)
			tc.mockSetup(mockBankKeeper)

			bankWrapper := wrappers.NewBankWrapper(mockBankKeeper)

			// When calling the function with a denom different than the evm one, it should panic
			defer func() {
				if r := recover(); r != nil {
					require.Contains(t, fmt.Sprint(r), tc.expPanic)
				}
			}()

			balance := bankWrapper.GetBalance(context.Background(), account, tc.evmDenom)

			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expCoin, balance, "expected a different balance")
			}
		})
	}
}

// ----------------------------------------QUERIES-------------------------------------------------

func TestSppendableCoin(t *testing.T) {
	eighteenDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]
	sixDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.SixDecimalsChainID]

	maxInt64 := int64(9223372036854775807)
	account := sdk.AccAddress([]byte("test_address"))

	testCases := []struct {
		name      string
		coinInfo  evmtypes.EvmCoinInfo
		evmDenom  string
		expCoin   sdk.Coin
		expErr    string
		expPanic  string
		mockSetup func(*testutil.MockBankWrapper)
	}{
		{
			name:     "success - convert 6 decimals amount to 18 decimals",
			coinInfo: sixDecimalsCoinInfo,
			evmDenom: sixDecimalsCoinInfo.Denom,
			expCoin:  sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e18)),
			expErr:   "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e18))

				mbk.EXPECT().
					SpendableCoin(
						gomock.Any(),
						account,
						sixDecimalsCoinInfo.ExtendedDenom,
					).Return(returnedCoin)
			},
		},
		{
			name:     "success - convert max int 6 decimals amount to 18 decimals",
			coinInfo: sixDecimalsCoinInfo,
			evmDenom: sixDecimalsCoinInfo.Denom,
			expCoin:  sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e12).MulRaw(maxInt64)),
			expErr:   "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e12).MulRaw(maxInt64))

				mbk.EXPECT().
					SpendableCoin(
						gomock.Any(),
						account,
						sixDecimalsCoinInfo.ExtendedDenom,
					).Return(returnedCoin)
			},
		},
		{
			name:     "success - does not convert 18 decimals amount",
			coinInfo: eighteenDecimalsCoinInfo,
			evmDenom: eighteenDecimalsCoinInfo.Denom,
			expCoin:  sdk.NewCoin(eighteenDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18)),
			expErr:   "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin(eighteenDecimalsCoinInfo.Denom, sdkmath.NewInt(1e18))

				mbk.EXPECT().
					SpendableCoin(
						gomock.Any(),
						account,
						eighteenDecimalsCoinInfo.Denom,
					).Return(returnedCoin)
			},
		},
		{
			name:     "success - zero balance",
			coinInfo: sixDecimalsCoinInfo,
			evmDenom: sixDecimalsCoinInfo.Denom,
			expCoin:  sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(0)),
			expErr:   "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(0))

				mbk.EXPECT().
					SpendableCoin(
						gomock.Any(),
						account,
						sixDecimalsCoinInfo.ExtendedDenom,
					).Return(returnedCoin)
			},
		},
		{
			name:     "success - small amount (less than 1 full token)",
			coinInfo: sixDecimalsCoinInfo,
			evmDenom: sixDecimalsCoinInfo.Denom,
			expCoin:  sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e14)), // 0.0001 token in 18 decimals
			expErr:   "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin(sixDecimalsCoinInfo.ExtendedDenom, sdkmath.NewInt(1e14)) // 0.0001 token in 6 decimals

				mbk.EXPECT().
					SpendableCoin(
						gomock.Any(),
						account,
						sixDecimalsCoinInfo.ExtendedDenom,
					).Return(returnedCoin)
			},
		},
		{
			name:      "panic - wrong evm denom",
			coinInfo:  eighteenDecimalsCoinInfo,
			evmDenom:  "wrong_denom",
			expPanic:  "expected evm denom",
			mockSetup: func(mbk *testutil.MockBankWrapper) {},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup EVM configurator to have access to the EVM coin info.
			configurator := evmtypes.NewEVMConfigurator()
			configurator.ResetTestConfig()
			err := configurator.WithEVMCoinInfo(tc.coinInfo).Configure()
			require.NoError(t, err, "failed to configure EVMConfigurator")

			// Setup mock controller
			ctrl := gomock.NewController(t)

			mockBankKeeper := testutil.NewMockBankWrapper(ctrl)
			tc.mockSetup(mockBankKeeper)

			bankWrapper := wrappers.NewBankWrapper(mockBankKeeper)

			// When calling the function with a denom different than the evm one, it should panic
			defer func() {
				if r := recover(); r != nil {
					require.Contains(t, fmt.Sprint(r), tc.expPanic)
				}
			}()

			balance := bankWrapper.SpendableCoin(context.Background(), account, tc.evmDenom)

			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expCoin, balance, "expected a different balance")
			}
		})
	}
}
