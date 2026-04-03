package bank

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/precompiles/bank"
	cmn "github.com/cosmos/evm/precompiles/common"
	precompiletestutil "github.com/cosmos/evm/precompiles/testutil"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	cosmosevmutiltx "github.com/cosmos/evm/testutil/tx"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *PrecompileTestSuite) TestBalances() {
	var ctx sdk.Context
	// setup test in order to have s.precompile, s.cosmosEVMAddr and s.xmplAddr defined
	s.SetupTest()
	method := s.precompile.Methods[bank.BalancesMethod]

	testcases := []struct {
		name        string
		malleate    func() []interface{}
		expPass     bool
		wantErr     error
		expBalances func(cosmosEVMAddr, xmplAddr common.Address) []bank.Balance
	}{
		{
			"fail - invalid number of arguments",
			func() []interface{} {
				return []interface{}{
					"", "",
				}
			},
			false,
			cmn.NewRevertWithSolidityError(bank.ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(1), big.NewInt(2)),
			nil,
		},
		{
			"fail - invalid account address",
			func() []interface{} {
				return []interface{}{
					"random text",
				}
			},
			false,
			cmn.NewRevertWithSolidityError(bank.ABI, cmn.SolidityErrInvalidAddress, "random text"),
			nil,
		},
		{
			"pass - empty balances for new account",
			func() []interface{} {
				return []interface{}{
					cosmosevmutiltx.GenerateAddress(),
				}
			},
			true,
			nil,
			func(common.Address, common.Address) []bank.Balance { return []bank.Balance{} },
		},
		{
			"pass - Initial balances present",
			func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(0),
				}
			},
			true,
			nil,
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
			func() []interface{} {
				ctx = s.mintAndSendXMPLCoin(ctx, s.keyring.GetAccAddr(0), math.NewInt(1e18))
				return []interface{}{
					s.keyring.GetAddr(0),
				}
			},
			true,
			nil,
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

			args := tc.malleate()
			bz, err := s.precompile.Balances(ctx, &method, args)

			if tc.expPass {
				s.Require().NoError(err)
				var balances []bank.Balance
				err = s.precompile.UnpackIntoInterface(&balances, method.Name, bz)
				s.Require().NoError(err)
				s.Require().Equal(tc.expBalances(s.cosmosEVMAddr, s.xmplAddr), balances)
			} else {
				s.Require().Error(err)
				s.Require().NotNil(tc.wantErr)
				precompiletestutil.RequireExactError(s.T(), err, tc.wantErr)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestTotalSupply() {
	var ctx sdk.Context
	// setup test in order to have s.precompile, s.cosmosEVMAddr and s.xmplAddr defined
	s.SetupTest()
	method := s.precompile.Methods[bank.TotalSupplyMethod]

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
			bz, err := s.precompile.TotalSupply(
				ctx,
				&method,
				nil,
			)

			s.Require().NoError(err)
			var balances []bank.Balance
			err = s.precompile.UnpackIntoInterface(&balances, method.Name, bz)
			s.Require().NoError(err)
			s.Require().Equal(tc.expSupply(s.cosmosEVMAddr, s.xmplAddr), balances)
		})
	}
}

func (s *PrecompileTestSuite) TestSupplyOf() {
	// setup test in order to have s.precompile, s.cosmosEVMAddr and s.xmplAddr defined
	s.SetupTest()
	method := s.precompile.Methods[bank.SupplyOfMethod]

	totSupplRes, err := s.grpcHandler.GetTotalSupply()
	s.Require().NoError(err)
	cosmosEVMTotalSupply := totSupplRes.Supply.AmountOf(s.bondDenom)
	xmplTotalSupply := totSupplRes.Supply.AmountOf(s.tokenDenom)

	testcases := []struct {
		name      string
		malleate  func() []interface{}
		expErr    bool
		wantErr   error
		expSupply *big.Int
	}{
		{
			"fail - invalid number of arguments",
			func() []interface{} {
				return []interface{}{
					"", "", "",
				}
			},
			true,
			cmn.NewRevertWithSolidityError(bank.ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(1), big.NewInt(3)),
			nil,
		},
		{
			"fail - invalid hex address",
			func() []interface{} {
				return []interface{}{
					"random text",
				}
			},
			true,
			cmn.NewRevertWithSolidityError(bank.ABI, cmn.SolidityErrInvalidAddress, "random text"),
			nil,
		},
		{
			"pass - erc20 not registered return 0 supply",
			func() []interface{} {
				return []interface{}{
					cosmosevmutiltx.GenerateAddress(),
				}
			},
			false,
			nil,
			big.NewInt(0),
		},
		{
			"pass - XMPL total supply",
			func() []interface{} {
				return []interface{}{
					s.xmplAddr,
				}
			},
			false,
			nil,
			xmplTotalSupply.BigInt(),
		},

		{
			"pass - ATOM total supply",
			func() []interface{} {
				return []interface{}{
					s.cosmosEVMAddr,
				}
			},
			false,
			nil,
			cosmosEVMTotalSupply.BigInt(),
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			ctx := s.SetupTest()

			args := tc.malleate()
			bz, err := s.precompile.SupplyOf(ctx, &method, args)

			if tc.expErr {
				s.Require().Error(err)
				s.Require().NotNil(tc.wantErr)
				precompiletestutil.RequireExactError(s.T(), err, tc.wantErr)
			} else {
				out, err := method.Outputs.Unpack(bz)
				s.Require().NoError(err, "expected no error unpacking")
				supply, ok := out[0].(*big.Int)
				s.Require().True(ok, "expected output to be a big.Int")
				s.Require().NoError(err)
				s.Require().Equal(supply.Int64(), tc.expSupply.Int64())
			}
		})
	}
}
