package network_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	testconstants "github.com/cosmos/evm/testutil/constants"
	grpchandler "github.com/cosmos/evm/testutil/integration/os/grpc"
	testkeyring "github.com/cosmos/evm/testutil/integration/os/keyring"
	"github.com/cosmos/evm/testutil/integration/os/network"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func TestWithChainID(t *testing.T) {
	eighteenDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]
	sixDecimalsCoinInfo := testconstants.ExampleChainCoinInfo[testconstants.SixDecimalsChainID]

	testCases := []struct {
		name            string
		chainID         string
		coinInfo        evmtypes.EvmCoinInfo
		expBaseFee      math.LegacyDec
		expCosmosAmount math.Int
	}{
		{
			name:            "18 decimals",
			chainID:         testconstants.ExampleChainID,
			coinInfo:        eighteenDecimalsCoinInfo,
			expBaseFee:      math.LegacyNewDec(875_000_000),
			expCosmosAmount: network.GetInitialAmount(evmtypes.EighteenDecimals),
		},
		{
			name:            "6 decimals",
			chainID:         testconstants.SixDecimalsChainID,
			coinInfo:        sixDecimalsCoinInfo,
			expBaseFee:      math.LegacyNewDecWithPrec(875, 6),
			expCosmosAmount: network.GetInitialAmount(evmtypes.SixDecimals),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new network with 2 pre-funded accounts
			keyring := testkeyring.New(1)

			opts := []network.ConfigOption{
				network.WithChainID(tc.chainID),
				network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
			}

			nw := network.New(opts...)

			handler := grpchandler.NewIntegrationHandler(nw)

			// ------------------------------------------------------------------------------------
			// Checks on initial balances.
			// ------------------------------------------------------------------------------------

			// Evm balance should always be in 18 decimals regardless of the
			// chain ID.

			// Evm balance should always be in 18 decimals
			req, err := handler.GetBalanceFromEVM(keyring.GetAccAddr(0))
			require.NoError(t, err, "error getting balances")
			require.Equal(t,
				network.GetInitialAmount(evmtypes.EighteenDecimals).String(),
				req.Balance,
				"expected amount to be in 18 decimals",
			)

			// Bank balance should always be in the original amount.
			cReq, err := handler.GetBalanceFromBank(keyring.GetAccAddr(0), tc.coinInfo.Denom)
			require.NoError(t, err, "error getting balances")
			require.Equal(t,
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
			require.NoError(t, err, "error getting base fee")
			require.Equal(t,
				tc.expBaseFee.String(),
				bfResp.BaseFee.String(),
				"expected amount to be in 18 decimals",
			)
		})
	}
}

func TestWithBalances(t *testing.T) {
	key1Balance := sdk.NewCoins(sdk.NewInt64Coin(testconstants.ExampleAttoDenom, 1e18))
	key2Balance := sdk.NewCoins(
		sdk.NewInt64Coin(testconstants.ExampleAttoDenom, 2e18),
		sdk.NewInt64Coin("other", 3e18),
	)

	// Create a new network with 2 pre-funded accounts
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
	nw := network.New(
		network.WithBalances(balances...),
	)
	handler := grpchandler.NewIntegrationHandler(nw)

	req, err := handler.GetAllBalances(keyring.GetAccAddr(0))
	require.NoError(t, err, "error getting balances")
	require.Len(t, req.Balances, 1, "wrong number of balances")
	require.Equal(t, balances[0].Coins, req.Balances, "wrong balances")

	req, err = handler.GetAllBalances(keyring.GetAccAddr(1))
	require.NoError(t, err, "error getting balances")
	require.Len(t, req.Balances, 2, "wrong number of balances")
	require.Equal(t, balances[1].Coins, req.Balances, "wrong balances")
}
