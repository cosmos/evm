package wrappers_test

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/evm/x/vm/wrappers"
	"github.com/cosmos/evm/x/vm/wrappers/testutil"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// --------------------------------------TRANSACTIONS-----------------------------------------------

const TokenDenom = "token"

func TestMintAmountToAccount(t *testing.T) {
	testCases := []struct {
		name        string
		evmDenom    string
		evmDecimals uint8
		amount      *big.Int
		recipient   sdk.AccAddress
		expectErr   string
		mockSetup   func(*testutil.MockBankWrapper)
	}{
		{
			name:        "success - convert 18 decimals amount to 6 decimals",
			evmDenom:    TokenDenom,
			evmDecimals: 6,
			amount:      big.NewInt(1e18), // 1 token in 18 decimals
			recipient:   sdk.AccAddress([]byte("test_address")),
			expectErr:   "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoin := sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e6)) // 1 token in 6 decimals
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
			name:        "success - 18 decimals amount not modified",
			evmDenom:    TokenDenom,
			evmDecimals: 18,
			amount:      big.NewInt(1e18), // 1 token in 18 decimals
			recipient:   sdk.AccAddress([]byte("test_address")),
			expectErr:   "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoin := sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e18))
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
			name:        "fail - mint coins error",
			evmDenom:    TokenDenom,
			evmDecimals: 6,
			amount:      big.NewInt(1e18),
			recipient:   sdk.AccAddress([]byte("test_address")),
			expectErr:   "failed to mint coins to account in bank wrapper",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoin := sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e6))
				expectedCoins := sdk.NewCoins(expectedCoin)

				mbk.EXPECT().
					MintCoins(gomock.Any(), evmtypes.ModuleName, expectedCoins).
					Return(errors.New("mint error"))
			},
		},
		{
			name:        "fail - send coins error",
			evmDenom:    "evm",
			evmDecimals: 6,
			amount:      big.NewInt(1e18),
			recipient:   sdk.AccAddress([]byte("test_address")),
			expectErr:   "send error",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoin := sdk.NewCoin("evm", sdkmath.NewInt(1e6))
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
					).Return(errors.New("send error"))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup EVM configurator to have access to the EVM coin info.
			configurator := evmtypes.NewEVMConfigurator()
			configurator.ResetTestConfig()
			err := configurator.WithEVMCoinInfo(tc.evmDenom, tc.evmDecimals).Configure()
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
	account := sdk.AccAddress([]byte("test_address"))

	testCases := []struct {
		name        string
		evmDenom    string
		evmDecimals uint8
		amount      *big.Int
		expectErr   string
		mockSetup   func(*testutil.MockBankWrapper)
	}{
		{
			name:        "success - convert 18 decimals amount to 6 decimals",
			evmDenom:    TokenDenom,
			evmDecimals: 6,
			amount:      big.NewInt(1e18),
			expectErr:   "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoin := sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e6))
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
			name:        "success - 18 decimals amount not modified",
			evmDenom:    TokenDenom,
			evmDecimals: 18,
			amount:      big.NewInt(1e18),
			expectErr:   "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoin := sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e18))
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
			name:        "fail - send coins error",
			evmDenom:    TokenDenom,
			evmDecimals: 6,
			amount:      big.NewInt(1e18),
			expectErr:   "failed to burn coins from account in bank wrapper",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoin := sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e6))
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
			name:        "fail - send burn error",
			evmDenom:    TokenDenom,
			evmDecimals: 6,
			amount:      big.NewInt(1e18),
			expectErr:   "burn error",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoin := sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e6))
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
			err := configurator.WithEVMCoinInfo(tc.evmDenom, tc.evmDecimals).Configure()
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
	account := sdk.AccAddress([]byte("test_address"))

	testCases := []struct {
		name        string
		evmDenom    string
		evmDecimals uint8
		coins       func() sdk.Coins
		expectErr   string
		mockSetup   func(*testutil.MockBankWrapper)
	}{
		{
			name:        "success - does not convert 18 decimals amount with single token",
			evmDenom:    TokenDenom,
			evmDecimals: 18,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e18)),
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
			name:        "success - convert 18 decimals amount to 6 decimals with single token",
			evmDenom:    TokenDenom,
			evmDecimals: 6,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e6)),
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
			name:        "success - does not convert 18 decimals amount with multiple tokens",
			evmDenom:    TokenDenom,
			evmDecimals: 18,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e18)),
					sdk.NewCoin("something", sdkmath.NewInt(3e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e18)),
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
			name:        "success - convert 18 decimals amount to 6 decimals with multiple tokens",
			evmDenom:    TokenDenom,
			evmDecimals: 6,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e18)),
					sdk.NewCoin("something", sdkmath.NewInt(3e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e6)),
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
			name:        "success - no op if converted coin is zero",
			evmDenom:    TokenDenom,
			evmDecimals: 6,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e11)),
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
			err := configurator.WithEVMCoinInfo(tc.evmDenom, tc.evmDecimals).Configure()
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
	account := sdk.AccAddress([]byte("test_address"))

	testCases := []struct {
		name        string
		evmDenom    string
		evmDecimals uint8
		coins       func() sdk.Coins
		expectErr   string
		mockSetup   func(*testutil.MockBankWrapper)
	}{
		{
			name:        "success - does not convert 18 decimals amount with single token",
			evmDenom:    TokenDenom,
			evmDecimals: 18,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e18)),
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
			name:        "success - convert 18 decimals amount to 6 decimals with single token",
			evmDenom:    TokenDenom,
			evmDecimals: 6,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e6)),
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
			name:        "success - does not convert 18 decimals amount with multiple tokens",
			evmDenom:    TokenDenom,
			evmDecimals: 18,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e18)),
					sdk.NewCoin("something", sdkmath.NewInt(3e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e18)),
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
			name:        "success - convert 18 decimals amount to 6 decimals with multiple tokens",
			evmDenom:    TokenDenom,
			evmDecimals: 6,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e18)),
					sdk.NewCoin("something", sdkmath.NewInt(3e18)),
				}...)
				return coins
			},
			expectErr: "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				expectedCoins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e6)),
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
			name:        "success - no op if converted coin is zero",
			evmDenom:    TokenDenom,
			evmDecimals: 6,
			coins: func() sdk.Coins {
				coins := sdk.NewCoins([]sdk.Coin{
					sdk.NewCoin(TokenDenom, sdkmath.NewInt(1e11)),
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
			err := configurator.WithEVMCoinInfo(tc.evmDenom, tc.evmDecimals).Configure()
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
	maxInt64 := int64(9223372036854775807)
	evmDenom := "token"
	account := sdk.AccAddress([]byte("test_address"))

	testCases := []struct {
		name        string
		evmDecimals uint8
		denom       string
		expCoin     sdk.Coin
		expErr      string
		expPanic    string
		mockSetup   func(*testutil.MockBankWrapper)
	}{
		{
			name:        "success - convert 6 decimals amount to 18 decimals",
			denom:       evmDenom,
			evmDecimals: 6,
			expCoin:     sdk.NewCoin(evmDenom, sdkmath.NewInt(1e18)),
			expErr:      "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin(evmDenom, sdkmath.NewInt(1e6))

				mbk.EXPECT().
					GetBalance(
						gomock.Any(),
						account,
						evmDenom,
					).Return(returnedCoin)
			},
		},
		{
			name:        "success - convert max int 6 decimals amount to 18 decimals",
			denom:       evmDenom,
			evmDecimals: 6,
			expCoin:     sdk.NewCoin(evmDenom, sdkmath.NewInt(1e12).MulRaw(maxInt64)),
			expErr:      "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin(evmDenom, sdkmath.NewInt(maxInt64))

				mbk.EXPECT().
					GetBalance(
						gomock.Any(),
						account,
						evmDenom,
					).Return(returnedCoin)
			},
		},
		{
			name:        "success - does not convert 18 decimals amount",
			denom:       evmDenom,
			evmDecimals: 18,
			expCoin:     sdk.NewCoin(evmDenom, sdkmath.NewInt(1e18)),
			expErr:      "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin(evmDenom, sdkmath.NewInt(1e18))

				mbk.EXPECT().
					GetBalance(
						gomock.Any(),
						account,
						evmDenom,
					).Return(returnedCoin)
			},
		},
		{
			name:        "success - zero balance",
			denom:       evmDenom,
			evmDecimals: 6,
			expCoin:     sdk.NewCoin(evmDenom, sdkmath.NewInt(0)),
			expErr:      "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin(evmDenom, sdkmath.NewInt(0))

				mbk.EXPECT().
					GetBalance(
						gomock.Any(),
						account,
						evmDenom,
					).Return(returnedCoin)
			},
		},
		{
			name:        "success - small amount (less than 1 full token)",
			denom:       evmDenom,
			evmDecimals: 6,
			expCoin:     sdk.NewCoin(evmDenom, sdkmath.NewInt(1e14)), // 0.0001 token in 18 decimals
			expErr:      "",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin(evmDenom, sdkmath.NewInt(100)) // 0.0001 token in 6 decimals

				mbk.EXPECT().
					GetBalance(
						gomock.Any(),
						account,
						evmDenom,
					).Return(returnedCoin)
			},
		},
		{
			name:        "panic - wrong evm denom",
			denom:       "wrong_denom",
			evmDecimals: 18,
			expPanic:    "expected evm denom token",
			mockSetup: func(mbk *testutil.MockBankWrapper) {
				returnedCoin := sdk.NewCoin("wrong_denom", sdkmath.NewInt(1e18))

				mbk.EXPECT().
					GetBalance(
						gomock.Any(),
						account,
						"wrong_denom",
					).Return(returnedCoin)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup EVM configurator to have access to the EVM coin info.
			configurator := evmtypes.NewEVMConfigurator()
			configurator.ResetTestConfig()
			err := configurator.WithEVMCoinInfo(evmDenom, tc.evmDecimals).Configure()
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

			balance := bankWrapper.GetBalance(context.Background(), account, tc.denom)

			if tc.expErr != "" {
				require.ErrorContains(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expCoin, balance, "expected a different balance")
			}
		})
	}
}
