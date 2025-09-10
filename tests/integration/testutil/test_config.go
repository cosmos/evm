//go:build test

package testutil

import (
	testconfig "github.com/cosmos/evm/testutil/config"
	grpchandler "github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// Note: TestSuite is defined in test_suite.go

func (s *TestSuite) TestWithChainID() {
	eighteenDecimalsConfig := testconfig.DefaultChainConfig
	sixDecimalsConfig := testconfig.SixDecimalsChainConfig

	testCases := []struct {
		name            string
		chainID         string
		evmChainID      uint64
		coinInfo        evmtypes.EvmCoinInfo
		expBaseFee      math.LegacyDec
		expCosmosAmount math.Int
	}{
		{
			name:            "18 decimals",
			chainID:         eighteenDecimalsConfig.ChainID,
			coinInfo:        *eighteenDecimalsConfig.EvmConfig.CoinInfo,
			expBaseFee:      math.LegacyNewDec(875_000_000),
			expCosmosAmount: network.GetInitialAmount(evmtypes.EighteenDecimals),
		},
		{
			name:            "6 decimals",
			chainID:         sixDecimalsConfig.ChainID,
			coinInfo:        *sixDecimalsConfig.EvmConfig.CoinInfo,
			expBaseFee:      math.LegacyNewDecWithPrec(875, 6),
			expCosmosAmount: network.GetInitialAmount(evmtypes.SixDecimals),
		},
	}

	for _, tc := range testCases {
		// create a new network with 2 pre-funded accounts
		keyring := testkeyring.New(1)

		// Create chain config for the test case
		chainConfig := testconfig.CreateChainConfig(tc.chainID, tc.evmChainID, nil, "test", tc.coinInfo.Decimals, tc.coinInfo.ExtendedDecimals)
		options := []network.ConfigOption{
			network.WithChainConfig(chainConfig),
			network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
		}
		options = append(options, s.options...)

		nw := network.New(s.create, options...)

		handler := grpchandler.NewIntegrationHandler(nw)

		// ------------------------------------------------------------------------------------
		// Checks on initial balances.
		// ------------------------------------------------------------------------------------

		// Evm balance should always be in 18 decimals regardless of the
		// chain ID.

		// Evm balance should always be in 18 decimals
		req, err := handler.GetBalanceFromEVM(keyring.GetAccAddr(0))
		s.NoError(err, "error getting balances")
		s.Equal(
			network.GetInitialAmount(evmtypes.EighteenDecimals).String(),
			req.Balance,
			"expected amount to be in 18 decimals",
		)

		// Bank balance should always be in the original amount.
		cReq, err := handler.GetBalanceFromBank(keyring.GetAccAddr(0), tc.coinInfo.GetDenom())
		s.NoError(err, "error getting balances")
		s.Equal(
			tc.expCosmosAmount.String(),
			cReq.Balance.Amount.String(),
			"expected amount to be in original decimals",
		)

		// ------------------------------------------------------------------------------------
		// Checks on the base fee.
		// ------------------------------------------------------------------------------------
		// Base fee should always be represented with the decimal
		// representation of the EVM denom coin.
		bfResp, err := handler.GetBaseFee()
		s.NoError(err, "error getting base fee")
		s.Equal(
			tc.expBaseFee.String(),
			bfResp.BaseFee.String(),
			"expected amount to be in 18 decimals",
		)
	}
}

func (s *TestSuite) TestWithBalances() {
	key1Balance := sdk.NewCoins(sdk.NewInt64Coin(testconfig.DefaultChainConfig.EvmConfig.CoinInfo.GetDenom(), 1e18))
	key2Balance := sdk.NewCoins(
		sdk.NewInt64Coin(testconfig.DefaultChainConfig.EvmConfig.Denom, 2e18),
		sdk.NewInt64Coin("other", 3e18),
	)

	// create a new network with 2 pre-funded accounts
	keyring := testkeyring.New(2)
	balances := []banktypes.Balance{
		{
			Address: keyring.GetAccAddr(0).String(),
			Coins:   key1Balance,
		},
		{
			Address: keyring.GetAccAddr(1).String(),
			Coins:   key2Balance,
		},
	}
	options := []network.ConfigOption{
		network.WithBalances(balances...),
	}
	options = append(options, s.options...)
	nw := network.New(s.create, options...)
	handler := grpchandler.NewIntegrationHandler(nw)

	req, err := handler.GetAllBalances(keyring.GetAccAddr(0))
	s.NoError(err, "error getting balances")
	s.Len(req.Balances, 1, "wrong number of balances")
	s.Equal(balances[0].Coins, req.Balances, "wrong balances")

	req, err = handler.GetAllBalances(keyring.GetAccAddr(1))
	s.NoError(err, "error getting balances")
	s.Len(req.Balances, 2, "wrong number of balances")
	s.Equal(balances[1].Coins, req.Balances, "wrong balances")
}
