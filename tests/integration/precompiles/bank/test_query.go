package bank

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/precompiles/bank"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	cosmosevmutiltx "github.com/cosmos/evm/testutil/tx"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *PrecompileTestSuite) TestBalances() {
	var ctx sdk.Context
	// setup test in order to have s.precompile, s.cosmosEVMAddr and s.xmplAddr defined
	s.SetupTest()

	testcases := []struct {
		name        string
		malleate    func() *bank.BalancesCall
		expPass     bool
		errContains string
		expBalances func(cosmosEVMAddr, xmplAddr common.Address) []bank.Balance
	}{
		{
			"pass - empty balances for new account",
			func() *bank.BalancesCall {
				return bank.NewBalancesCall(
					cosmosevmutiltx.GenerateAddress(),
				)
			},
			true,
			"",
			func(common.Address, common.Address) []bank.Balance { return []bank.Balance{} },
		},
		{
			"pass - Initial balances present",
			func() *bank.BalancesCall {
				return bank.NewBalancesCall(
					s.keyring.GetAddr(0),
				)
			},
			true,
			"",
			func(cosmosEVMAddr, xmplAddr common.Address) []bank.Balance {
				return []bank.Balance{
					{
						ContractAddress: cosmosEVMAddr,
						Amount:          network.PrefundedAccountInitialBalance.BigInt(),
					},
					{
						ContractAddress: xmplAddr,
						Amount:          network.PrefundedAccountInitialBalance.BigInt(),
					},
				}
			},
		},
		{
			"pass - ATOM and XMPL balances present - mint extra XMPL",
			func() *bank.BalancesCall {
				ctx = s.mintAndSendXMPLCoin(ctx, s.keyring.GetAccAddr(0), math.NewInt(1e18))
				return bank.NewBalancesCall(
					s.keyring.GetAddr(0),
				)
			},
			true,
			"",
			func(cosmosEVMAddr, xmplAddr common.Address) []bank.Balance {
				return []bank.Balance{{
					ContractAddress: cosmosEVMAddr,
					Amount:          network.PrefundedAccountInitialBalance.BigInt(),
				}, {
					ContractAddress: xmplAddr,
					Amount:          network.PrefundedAccountInitialBalance.Add(math.NewInt(1e18)).BigInt(),
				}}
			},
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			ctx = s.SetupTest() // reset the chain each test

			out, err := s.precompile.Balances(
				ctx,
				*tc.malleate(),
			)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(tc.expBalances(s.cosmosEVMAddr, s.xmplAddr), out.Balances)
			} else {
				s.Require().Contains(err.Error(), tc.errContains)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestTotalSupply() {
	var ctx sdk.Context
	// setup test in order to have s.precompile, s.cosmosEVMAddr and s.xmplAddr defined
	s.SetupTest()

	totSupplRes, err := s.grpcHandler.GetTotalSupply()
	s.Require().NoError(err)
	cosmosEVMTotalSupply := totSupplRes.Supply.AmountOf(s.bondDenom)
	xmplTotalSupply := totSupplRes.Supply.AmountOf(s.tokenDenom)

	testcases := []struct {
		name      string
		malleate  func()
		expSupply func(cosmosEVMAddr, xmplAddr common.Address) []bank.Balance
	}{
		{
			"pass - ATOM and XMPL total supply",
			func() {
				ctx = s.mintAndSendXMPLCoin(ctx, s.keyring.GetAccAddr(0), math.NewInt(1e18))
			},
			func(cosmosEVMAddr, xmplAddr common.Address) []bank.Balance {
				return []bank.Balance{{
					ContractAddress: cosmosEVMAddr,
					Amount:          cosmosEVMTotalSupply.BigInt(),
				}, {
					ContractAddress: xmplAddr,
					Amount:          xmplTotalSupply.Add(math.NewInt(1e18)).BigInt(),
				}}
			},
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			ctx = s.SetupTest()
			tc.malleate()
			out, err := s.precompile.TotalSupply(
				ctx,
				bank.TotalSupplyCall{},
			)

			s.Require().NoError(err)
			s.Require().Equal(tc.expSupply(s.cosmosEVMAddr, s.xmplAddr), out.TotalSupply)
		})
	}
}

func (s *PrecompileTestSuite) TestSupplyOf() {
	// setup test in order to have s.precompile, s.cosmosEVMAddr and s.xmplAddr defined
	s.SetupTest()

	totSupplRes, err := s.grpcHandler.GetTotalSupply()
	s.Require().NoError(err)
	cosmosEVMTotalSupply := totSupplRes.Supply.AmountOf(s.bondDenom)
	xmplTotalSupply := totSupplRes.Supply.AmountOf(s.tokenDenom)

	testcases := []struct {
		name        string
		malleate    func() *bank.SupplyOfCall
		expErr      bool
		errContains string
		expSupply   *big.Int
	}{
		{
			"pass - erc20 not registered return 0 supply",
			func() *bank.SupplyOfCall {
				return bank.NewSupplyOfCall(
					cosmosevmutiltx.GenerateAddress(),
				)
			},
			false,
			"",
			big.NewInt(0),
		},
		{
			"pass - XMPL total supply",
			func() *bank.SupplyOfCall {
				return bank.NewSupplyOfCall(
					s.xmplAddr,
				)
			},
			false,
			"",
			xmplTotalSupply.BigInt(),
		},

		{
			"pass - ATOM total supply",
			func() *bank.SupplyOfCall {
				return bank.NewSupplyOfCall(
					s.cosmosEVMAddr,
				)
			},
			false,
			"",
			cosmosEVMTotalSupply.BigInt(),
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			ctx := s.SetupTest()

			out, err := s.precompile.SupplyOf(
				ctx,
				*tc.malleate(),
			)

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().Equal(out.TotalSupply.Int64(), tc.expSupply.Int64())
			}
		})
	}
}
