package erc20factory_test

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/precompiles/erc20factory"
	erc20types "github.com/cosmos/evm/x/erc20/types"
)

func (s *PrecompileTestSuite) TestCreate() {
	caller := common.HexToAddress("0x2c7882f69Cd115F470aAEde121f57F932936a56f")
	mintAddr := common.HexToAddress("0x73657398D483143AF7db7899757e5E7037fB713d")
	expectedAddress := common.HexToAddress("0x30E56567F73403eD713dA0b0419e4A5330A16896")
	amount := big.NewInt(1000000)
	decimals := uint8(18)
	name := "Test"
	symbol := "TEST"

	method := s.precompile.Methods[erc20factory.CreateMethod]

	testcases := []struct {
		name        string
		args        []interface{}
		expPass     bool
		postExpPass func(output []byte)
		errContains string
		expAddress  common.Address
	}{
		{
			name:    "pass - correct arguments",
			args:    []interface{}{uint8(0), [32]uint8(common.HexToHash("0x4f5b6f778b28c4d67a9c12345678901234567890123456789012345678901234").Bytes()), name, symbol, decimals, mintAddr, amount},
			expPass: true,
			postExpPass: func(output []byte) {
				res, err := method.Outputs.Unpack(output)
				s.Require().NoError(err, "expected no error unpacking output")
				s.Require().Len(res, 1, "expected one output")
				address, ok := res[0].(common.Address)
				s.Require().True(ok, "expected address type")

				// Check the balance of the token for the mintAddr
				balance := s.network.App.BankKeeper.GetBalance(s.network.GetContext(), sdk.AccAddress(mintAddr.Bytes()), erc20types.CreateDenom(address.String()))
				s.Require().Equal(amount, balance.Amount.BigInt(), "expected balance to match preminted amount")

				s.Require().Equal(address.String(), expectedAddress, "expected address to match")

			},
			expAddress: expectedAddress,
		},
		{
			name: "fail - invalid tokenType",
			args: []interface{}{
				"invalid tokenType",
				[32]uint8{},
				name,
				symbol,
				decimals,
				mintAddr,
				amount,
			},
			errContains: "invalid tokenType",
		},
		{
			name: "fail - invalid salt",
			args: []interface{}{
				uint8(0),
				"invalid salt",
				name,
				symbol,
				decimals,
				mintAddr,
				amount,
			},
			errContains: "invalid salt",
		},
		{
			name: "fail - invalid name",
			args: []interface{}{
				uint8(0),
				[32]uint8{},
				"",
				symbol,
				decimals,
				mintAddr,
				amount,
			},
			errContains: "invalid name",
		},
		{
			name: "fail - invalid symbol",
			args: []interface{}{
				uint8(0),
				[32]uint8{},
				name,
				"is",
				decimals,
				mintAddr,
				amount,
			},
			errContains: "invalid symbol",
		},
		{
			name: "fail - invalid decimals",
			args: []interface{}{
				uint8(0),
				[32]uint8{},
				name,
				symbol,
				"invalid decimals",
				mintAddr,
				amount,
			},
			errContains: "invalid decimals",
		},
		{
			name: "fail - invalid minter",
			args: []interface{}{
				uint8(0),
				[32]uint8{},
				name,
				symbol,
				decimals,
				"invalid address",
				amount,
			},
			errContains: "invalid minter",
		},
		{
			name: "fail - invalid preminted supply",
			args: []interface{}{
				uint8(0),
				[32]uint8{},
				name,
				symbol,
				decimals,
				mintAddr,
				"invalid amount",
			},
			errContains: "invalid premintedSupply",
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
			s.SetupTest()

			precompile := s.setupERC20FactoryPrecompile()

			method := precompile.Methods[erc20factory.CreateMethod]

			bz, err := precompile.Create(
				s.network.GetContext(),
				s.network.GetStateDB(),
				&method,
				caller,
				tc.args,
			)

			// NOTE: all output and error checking happens in here
			s.requireOut(bz, err, method, tc.expPass, tc.errContains, tc.expAddress)
		})
	}
}
