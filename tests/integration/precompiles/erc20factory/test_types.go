package erc20factory

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

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
				[32]uint8{},
			},
			expPass: true,
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
			salt, err := erc20factory.ParseCalculateAddressArgs(tc.args)
			if tc.expPass {
				s.Require().NoError(err, "unexpected error parsing the calculate address arguments")
				s.Require().Equal(salt, tc.args[0], "expected different salt")
			} else {
				s.Require().Error(err, "expected an error parsing the calculate address arguments")
				s.Require().ErrorContains(err, tc.errContains, "expected different error message")
			}
		})
	}
}

func (s *PrecompileTestSuite) TestParseCreateArgs() {
	addr := utiltx.GenerateAddress()
	decimals := uint8(18)
	amount := big.NewInt(1000000)
	name := "Test"
	symbol := "TEST"

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
				[32]uint8{},
				name,
				symbol,
				decimals,
				addr,
				amount,
			},
			expPass: true,
		},
		{
			name: "fail - invalid salt",
			args: []interface{}{
				"invalid salt",
				name,
				symbol,
				decimals,
				addr,
				big.NewInt(1000000),
			},
		},
		{
			name: "fail - invalid name",
			args: []interface{}{
				[32]uint8{},
				uint8(0),
				symbol,
				decimals,
				addr,
				big.NewInt(1000000),
			},
			errContains: "invalid name",
		},
		{
			name: "fail - invalid symbol",
			args: []interface{}{
				[32]uint8{},
				name,
				"",
				decimals,
				addr,
				big.NewInt(1000000),
			},
			errContains: "invalid symbol",
		},
		{
			name: "fail - invalid decimals",
			args: []interface{}{
				[32]uint8{},
				name,
				symbol,
				"invalid decimals",
				addr,
				big.NewInt(1000000),
			},
			errContains: "invalid decimals",
		},
		{
			name: "fail - invalid minter",
			args: []interface{}{
				[32]uint8{},
				name,
				symbol,
				decimals,
				"invalid address",
				big.NewInt(1000000),
			},
			errContains: "invalid minter",
		},
		{
			name: "fail - zero address minter",
			args: []interface{}{
				[32]uint8{},
				name,
				symbol,
				decimals,
				common.Address{}, // Zero address
				big.NewInt(1000000),
			},
			errContains: "invalid minter: cannot be zero address",
		},
		{
			name: "fail - invalid preminted supply",
			args: []interface{}{
				[32]uint8{},
				name,
				symbol,
				decimals,
				addr,
				big.NewInt(-1),
			},
			errContains: "invalid premintedSupply: cannot be negative",
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
			salt, name, symbol, decimals, minter, premintedSupply, err := erc20factory.ParseCreateArgs(tc.args)
			if tc.expPass {
				s.Require().NoError(err, "unexpected error parsing the create arguments")
				s.Require().Equal(salt, tc.args[0], "expected different salt")
				s.Require().Equal(name, tc.args[1], "expected different name")
				s.Require().Equal(symbol, tc.args[2], "expected different symbol")
				s.Require().Equal(decimals, tc.args[3], "expected different decimals")
				s.Require().Equal(minter, tc.args[4], "expected different minter")
				s.Require().Equal(premintedSupply, tc.args[5], "expected different preminted supply")
			} else {
				s.Require().Error(err, "expected an error parsing the create arguments")
				s.Require().ErrorContains(err, tc.errContains, "expected different error message")
			}
		})
	}
}
