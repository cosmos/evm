package erc20factory_test

import (
	"math/big"

	"github.com/cosmos/evm/precompiles/erc20factory"
	utiltx "github.com/cosmos/evm/testutil/tx"
)

func (s *PrecompileTestSuite) TestIsTransaction() {
	s.SetupTest()

	// Queries
	method := s.precompile.Methods[erc20factory.CalculateAddressMethod]
	s.Require().False(s.precompile.IsTransaction(&method))

	// Transactions
	method = s.precompile.Methods[erc20factory.CreateMethod]
	s.Require().True(s.precompile.IsTransaction(&method))
}

func (s *PrecompileTestSuite) TestRequiredGas() {
	s.SetupTest()

	mintAddr := utiltx.GenerateAddress()
	decimals := uint8(18)
	amount := big.NewInt(1000000)
	name := "Test"
	symbol := "TEST"

	testcases := []struct {
		name     string
		malleate func() []byte
		expGas   uint64
	}{
		{
			name: erc20factory.CalculateAddressMethod,
			malleate: func() []byte {
				bz, err := s.precompile.Pack(erc20factory.CalculateAddressMethod, uint8(0), [32]uint8{})
				s.Require().NoError(err, "expected no error packing ABI")
				return bz
			},
			expGas: erc20factory.GasCalculateAddress,
		},
		{
			name: erc20factory.CreateMethod,
			malleate: func() []byte {
				bz, err := s.precompile.Pack(erc20factory.CreateMethod, uint8(0), [32]uint8{}, name, symbol, decimals, mintAddr, amount)
				s.Require().NoError(err, "expected no error packing ABI")
				return bz
			},
			expGas: erc20factory.GasCreate,
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			gas := s.precompile.RequiredGas(tc.malleate())
			s.Require().Equal(tc.expGas, gas)
		})
	}
}