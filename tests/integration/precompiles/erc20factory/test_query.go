package erc20factory

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/precompiles/erc20factory"
)

func (s *PrecompileTestSuite) TestCalculateAddress() {
	defaultCaller := common.HexToAddress("0xDc411BaFB148ebDA2B63EBD5f3D8669DD4383Af5")

	testcases := []struct {
		name        string
		caller      common.Address
		args        []interface{}
		expPass     bool
		errContains string
		expAddress  common.Address
	}{
		{
			name:   "pass - correct arguments",
			caller: defaultCaller,
			args: []interface{}{
				[32]uint8(common.HexToHash("0x4f5b6f778b28c4d67a9c12345678901234567890123456789012345678901234").Bytes()),
			},
			expPass:    true,
			expAddress: common.HexToAddress("0xc047E2F9302F4dE42115E40CEdb3FA0F1CfbD6b7"),
		},
		{
			name:   "fail - invalid salt",
			caller: defaultCaller,
			args: []interface{}{
				"invalid salt",
			},
			errContains: "invalid salt",
		},
		{
			name:   "fail - invalid number of arguments",
			caller: defaultCaller,
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

			method := precompile.Methods[erc20factory.CalculateAddressMethod]

			bz, err := precompile.CalculateAddress(
				&method,
				tc.caller,
				tc.args,
			)

			// NOTE: all output and error checking happens in here
			s.requireOut(bz, err, method, tc.expPass, tc.errContains, tc.expAddress)
		})
	}
}
