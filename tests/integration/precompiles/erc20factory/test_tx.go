package erc20factory

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/precompiles/erc20factory"
)

func (s *PrecompileTestSuite) TestCreate() {
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
			args:    []interface{}{uint8(0), [32]uint8(common.HexToHash("0x4f5b6f778b28c4d67a9c12345678901234567890123456789012345678901234").Bytes()), "AAA", "aaa", uint8(3)},
			expPass: true,
			postExpPass: func(output []byte) {
				res, err := method.Outputs.Unpack(output)
				s.Require().NoError(err, "expected no error unpacking output")
				s.Require().Len(res, 1, "expected one output")
				address, ok := res[0].(common.Address)
				s.Require().True(ok, "expected address type")
				s.Require().Equal(address.String(), "0x737F1dD6B32Bd863251F88a25489D8e18999F74a", "expected address to match")
			},
			expAddress: common.HexToAddress("0x737F1dD6B32Bd863251F88a25489D8e18999F74a"),
		},
		{
			name: "fail - invalid tokenType",
			args: []interface{}{
				"invalid tokenType",
				[32]uint8{},
				"Test",
				"TEST",
				uint8(18),
			},
			errContains: "invalid tokenType",
		},
		{
			name: "fail - invalid salt",
			args: []interface{}{
				uint8(0),
				"invalid salt",
				"Test",
				"TEST",
				uint8(18),
			},
			errContains: "invalid salt",
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
				&method,
				common.HexToAddress("0x0000000000000000000000000000000000000000"),
				tc.args,
			)

			// NOTE: all output and error checking happens in here
			s.requireOut(bz, err, method, tc.expPass, tc.errContains, tc.expAddress)
		})
	}
}
