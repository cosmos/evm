//go:build test

package testutil

import (
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/integration/evm/utils"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func (s *TestSuite) TestCheckBalances() {
	testDenom := "atest"
	keyring := testkeyring.New(1)
	address := keyring.GetAccAddr(0).String()

	testcases := []struct {
		name        string
		decimals    uint8
		expAmount   math.Int
		expPass     bool
		errContains string
	}{
		{
			name:      "pass - eighteen decimals",
			decimals:  18,
			expAmount: network.GetInitialAmount(evmtypes.EighteenDecimals),
			expPass:   true,
		},
		{
			name:      "pass - six decimals",
			decimals:  6,
			expAmount: network.GetInitialAmount(evmtypes.SixDecimals),
			expPass:   true,
		},
		{
			name:        "fail - wrong amount",
			decimals:    18,
			expAmount:   math.NewInt(1),
			errContains: "expected balance",
		},
	}

	for _, tc := range testcases {
		balances := []banktypes.Balance{{
			Address: address,
			Coins: sdk.NewCoins(
				sdk.NewCoin(testDenom, tc.expAmount),
			),
		}}

		options := []network.ConfigOption{
			network.WithBaseCoin(testDenom, tc.decimals),
			network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
		}
		options = append(options, s.options...)
		nw := network.New(s.create, options...)
		err := utils.CheckBalances(nw.GetContext(), nw.GetBankClient(), balances)
		if tc.expPass {
			s.NoError(err, "unexpected error checking balances")
		} else {
			s.Error(err, "expected error checking balances")
			s.ErrorContains(err, tc.errContains, "expected different error checking balances")
		}
	}
}
