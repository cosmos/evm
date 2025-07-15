package erc20factory

import (
	"math/big"

	"github.com/cosmos/evm/precompiles/erc20factory"
	utiltx "github.com/cosmos/evm/testutil/tx"
)

func (s *PrecompileTestSuite) TestParseCalculateAddressArgs() {
	s.SetupTest()

	testcases := []struct {
		name        string
		args        []interface{}
		expPass     bool
		errContains string
	}{
		{
			name: "pass - correct arguments",
			args: []interface{}{
				uint8(0),
				[32]uint8{},
			},
			expPass: true,
		},
		{
			name: "fail - invalid tokenType",
			args: []interface{}{
				"invalid tokenType",
				[32]uint8{},
			},
			errContains: "invalid tokenType",
		},
		{
			name: "fail - invalid salt",
			args: []interface{}{
				uint8(0),
				"invalid salt",
			},
		},
		{
			name: "fail - invalid number of arguments",
			args: []interface{}{
				1, 2, 3,
			},
			errContains: "invalid number of arguments",
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			tokenType, salt, err := erc20factory.ParseCalculateAddressArgs(tc.args)
			if tc.expPass {
				s.Require().NoError(err, "unexpected error parsing the calculate address arguments")
				s.Require().Equal(tokenType, tc.args[0], "expected different token type")
				s.Require().Equal(salt, tc.args[1], "expected different salt")
			} else {
				s.Require().Error(err, "expected an error parsing the calculate address arguments")
				s.Require().ErrorContains(err, tc.errContains, "expected different error message")
			}
		})
	}
}

func (s *PrecompileTestSuite) TestParseCreateArgs() {
	addr := utiltx.GenerateAddress()
	s.SetupTest()

	testcases := []struct {
		name        string
		args        []interface{}
		expPass     bool
		errContains string
	}{
		{
			name: "pass - correct arguments",
			args: []interface{}{
				uint8(0),
				[32]uint8{},
				"Test",
				"TEST",
				uint8(18),
				addr,
				big.NewInt(1000000),
			},
			expPass: true,
		},
		{
			name: "fail - invalid tokenType",
			args: []interface{}{
				"invalid tokenType",
				[32]uint8{},
				"Test",
				"TEST",
				uint8(18),
				addr,
				big.NewInt(1000000),
			},
		},
		{
			name: "fail - invalid salt",
			args: []interface{}{
				uint8(0),
				"invalid salt",
				"Test",
				"TEST",
				uint8(18),
				addr,
				big.NewInt(1000000),
			},
		},
		{
			name: "fail - invalid name",
			args: []interface{}{
				uint8(0),
				[32]uint8{},
				uint8(0),
				"TEST",
				uint8(18),
				addr,
				big.NewInt(1000000),
			},
			errContains: "invalid name",
		},
		{
			name: "fail - invalid symbol",
			args: []interface{}{
				uint8(0),
				[32]uint8{},
				"Test",
				"is",
				uint8(18),
				addr,
				big.NewInt(1000000),
			},
			errContains: "invalid symbol",
		},
		{
			name: "fail - invalid decimals",
			args: []interface{}{
				uint8(0),
				[32]uint8{},
				"Test",
				"TEST",
				"invalid decimals",
				addr,
				big.NewInt(1000000),
			},
			errContains: "invalid decimals",
		},
		{
			name: "fail - invalid minter",
			args: []interface{}{
				uint8(0),
				[32]uint8{},
				"Test",
				"TEST",
				uint8(18),
				"invalid address",
				big.NewInt(1000000),
			},
			errContains: "invalid minter",
		},
		{
			name: "fail - invalid preminted supply",
			args: []interface{}{
				uint8(0),
				[32]uint8{},
				"Test",
				"TEST",
				addr,
				big.NewInt(-1),
			},
			errContains: "invalid preminted supply",
		},
		{
			name: "fail - invalid number of arguments",
			args: []interface{}{
				1, 2, 3, 4, 5,
			},
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			tokenType, salt, name, symbol, decimals, minter, premintedSupply, err := erc20factory.ParseCreateArgs(tc.args)
			if tc.expPass {
				s.Require().NoError(err, "unexpected error parsing the create arguments")
				s.Require().Equal(tokenType, tc.args[0], "expected different token type")
				s.Require().Equal(salt, tc.args[1], "expected different salt")
				s.Require().Equal(name, tc.args[2], "expected different name")
				s.Require().Equal(symbol, tc.args[3], "expected different symbol")
				s.Require().Equal(decimals, tc.args[4], "expected different decimals")
				s.Require().Equal(minter, tc.args[5], "expected different minter")
				s.Require().Equal(premintedSupply, tc.args[6], "expected different preminted supply")
			} else {
				s.Require().Error(err, "expected an error parsing the create arguments")
				s.Require().ErrorContains(err, tc.errContains, "expected different error message")
			}
		})
	}
}
