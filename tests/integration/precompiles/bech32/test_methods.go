package bech32

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/precompiles/bech32"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *PrecompileTestSuite) TestHexToBech32() {
	// setup basic test suite
	s.SetupTest()

	testCases := []struct {
		name        string
		malleate    func() bech32.HexToBech32Call
		postCheck   func(result *bech32.HexToBech32Return)
		expError    bool
		errContains string
	}{
		{
			"fail - invalid hex address",
			func() bech32.HexToBech32Call {
				return bech32.HexToBech32Call{
					Addr:   common.Address{},
					Prefix: "",
				}
			},
			func(result *bech32.HexToBech32Return) {},
			true,
			"invalid bech32 human readable prefix (HRP)",
		},
		{
			"fail - invalid bech32 HRP",
			func() bech32.HexToBech32Call {
				return bech32.HexToBech32Call{
					Addr:   s.keyring.GetAddr(0),
					Prefix: "",
				}
			},
			func(result *bech32.HexToBech32Return) {},
			true,
			"invalid bech32 human readable prefix (HRP)",
		},
		{
			"pass - valid hex address and valid bech32 HRP",
			func() bech32.HexToBech32Call {
				return bech32.HexToBech32Call{
					Addr:   s.keyring.GetAddr(0),
					Prefix: "cosmos",
				}
			},
			func(result *bech32.HexToBech32Return) {
				s.Require().Equal(s.keyring.GetAccAddr(0).String(), result.Bech32Address)
			},
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			result, err := s.precompile.HexToBech32(tc.malleate())

			if tc.expError {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.errContains, err.Error())
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(result)
				tc.postCheck(result)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestBech32ToHex() {
	// setup basic test suite
	s.SetupTest()

	testCases := []struct {
		name        string
		malleate    func() bech32.Bech32ToHexCall
		postCheck   func(result *bech32.Bech32ToHexReturn)
		expError    bool
		errContains string
	}{
		{
			"fail - empty bech32 address",
			func() bech32.Bech32ToHexCall {
				return bech32.Bech32ToHexCall{
					Bech32Address: "",
				}
			},
			func(result *bech32.Bech32ToHexReturn) {},
			true,
			"invalid bech32 address",
		},
		{
			"fail - invalid bech32 address",
			func() bech32.Bech32ToHexCall {
				return bech32.Bech32ToHexCall{
					Bech32Address: "cosmos",
				}
			},
			func(result *bech32.Bech32ToHexReturn) {},
			true,
			fmt.Sprintf("invalid bech32 address: %s", "cosmos"),
		},
		{
			"fail - decoding bech32 failed",
			func() bech32.Bech32ToHexCall {
				return bech32.Bech32ToHexCall{
					Bech32Address: "cosmos" + "1",
				}
			},
			func(result *bech32.Bech32ToHexReturn) {},
			true,
			"decoding bech32 failed",
		},
		{
			"fail - invalid address format",
			func() bech32.Bech32ToHexCall {
				return bech32.Bech32ToHexCall{
					Bech32Address: sdk.AccAddress(make([]byte, 256)).String(),
				}
			},
			func(result *bech32.Bech32ToHexReturn) {},
			true,
			"address max length is 255",
		},
		{
			"success - valid bech32 address",
			func() bech32.Bech32ToHexCall {
				return bech32.Bech32ToHexCall{
					Bech32Address: s.keyring.GetAccAddr(0).String(),
				}
			},
			func(result *bech32.Bech32ToHexReturn) {
				s.Require().Equal(s.keyring.GetAddr(0), result.Addr)
			},
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			result, err := s.precompile.Bech32ToHex(tc.malleate())

			if tc.expError {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(result)
				tc.postCheck(result)
			}
		})
	}
}
