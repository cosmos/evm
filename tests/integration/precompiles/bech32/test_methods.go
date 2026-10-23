package bech32

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/precompiles/bech32"
	cmn "github.com/cosmos/evm/precompiles/common"
	precompiletestutil "github.com/cosmos/evm/precompiles/testutil"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *PrecompileTestSuite) TestHexToBech32() {
	// setup basic test suite
	s.SetupTest()

	method := s.precompile.Methods[bech32.HexToBech32Method]

	testCases := []struct {
		name      string
		malleate  func() []interface{}
		postCheck func(data []byte)
		expError  bool
		wantErr   error
	}{
		{
			"fail - invalid args length",
			func() []interface{} {
				return []interface{}{}
			},
			func([]byte) {},
			true,
			cmn.NewRevertWithSolidityError(bech32.ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(2), big.NewInt(0)),
		},
		{
			"fail - invalid hex address",
			func() []interface{} {
				return []interface{}{
					"",
					"",
				}
			},
			func([]byte) {},
			true,
			cmn.NewRevertWithSolidityError(bech32.ABI, cmn.SolidityErrInvalidAddress, ""),
		},
		{
			"fail - invalid bech32 HRP",
			func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(0),
					"",
				}
			},
			func([]byte) {},
			true,
			cmn.NewRevertWithSolidityError(
				bech32.ABI,
				cmn.SolidityErrInvalidAddress,
				fmt.Sprintf(
					"invalid HRP: empty; expected account (%s), validator (%s), or consensus (%s) style prefix",
					sdk.GetConfig().GetBech32AccountAddrPrefix(),
					sdk.GetConfig().GetBech32ValidatorAddrPrefix(),
					sdk.GetConfig().GetBech32ConsensusAddrPrefix(),
				),
			),
		},
		{
			"pass - valid hex address and valid bech32 HRP",
			func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(0),
					sdk.GetConfig().GetBech32AccountAddrPrefix(),
				}
			},
			func(data []byte) {
				args, err := s.precompile.Unpack(bech32.HexToBech32Method, data)
				s.Require().NoError(err, "failed to unpack output")
				s.Require().Len(args, 1)
				addr, ok := args[0].(string)
				s.Require().True(ok)
				s.Require().Equal(s.keyring.GetAccAddr(0).String(), addr)
			},
			false,
			nil,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			bz, err := s.precompile.HexToBech32(&method, tc.malleate())

			if tc.expError {
				s.Require().Error(err)
				s.Require().NotNil(tc.wantErr)
				precompiletestutil.RequireExactError(s.T(), err, tc.wantErr)
				s.Require().Empty(bz)
			} else {
				s.Require().NoError(err)
				s.Require().NotEmpty(bz)
				tc.postCheck(bz)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestBech32ToHex() {
	// setup basic test suite
	s.SetupTest()

	method := s.precompile.Methods[bech32.Bech32ToHexMethod]

	testCases := []struct {
		name      string
		malleate  func() []interface{}
		postCheck func(data []byte)
		expError  bool
		wantErr   func() error
	}{
		{
			"fail - invalid args length",
			func() []interface{} {
				return []interface{}{}
			},
			func([]byte) {},
			true,
			func() error {
				return cmn.NewRevertWithSolidityError(bech32.ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(1), big.NewInt(0))
			},
		},
		{
			"fail - empty bech32 address",
			func() []interface{} {
				return []interface{}{
					"",
				}
			},
			func([]byte) {},
			true,
			func() error {
				return cmn.NewRevertWithSolidityError(bech32.ABI, cmn.SolidityErrInvalidAddress, "")
			},
		},
		{
			"fail - invalid bech32 address",
			func() []interface{} {
				return []interface{}{
					"cosmos",
				}
			},
			func([]byte) {},
			true,
			func() error {
				return cmn.NewRevertWithSolidityError(bech32.ABI, cmn.SolidityErrInvalidAddress, "cosmos")
			},
		},
		{
			"fail - decoding bech32 failed",
			func() []interface{} {
				return []interface{}{
					"cosmos" + "1",
				}
			},
			func([]byte) {},
			true,
			func() error {
				// Keep exact match but derive the sdk error message from the same call path.
				_, err := sdk.GetFromBech32("cosmos1", "cosmos")
				s.Require().Error(err)
				return cmn.NewRevertWithSolidityError(bech32.ABI, cmn.SolidityErrQueryFailed, bech32.Bech32ToHexMethod, err.Error())
			},
		},
		{
			"fail - invalid address format",
			func() []interface{} {
				return []interface{}{
					sdk.AccAddress(make([]byte, 256)).String(),
				}
			},
			func([]byte) {},
			true,
			func() error {
				// VerifyAddressFormat error depends on configured verifier; derive dynamically.
				addressBz := sdk.AccAddress(make([]byte, 256))
				err := sdk.VerifyAddressFormat(addressBz)
				s.Require().Error(err)
				return cmn.NewRevertWithSolidityError(bech32.ABI, cmn.SolidityErrInvalidAddress, err.Error())
			},
		},
		{
			"success - valid bech32 address",
			func() []interface{} {
				return []interface{}{
					s.keyring.GetAccAddr(0).String(),
				}
			},
			func(data []byte) {
				args, err := s.precompile.Unpack(bech32.Bech32ToHexMethod, data)
				s.Require().NoError(err, "failed to unpack output")
				s.Require().Len(args, 1)
				addr, ok := args[0].(common.Address)
				s.Require().True(ok)
				s.Require().Equal(s.keyring.GetAddr(0), addr)
			},
			false,
			nil,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			bz, err := s.precompile.Bech32ToHex(&method, tc.malleate())

			if tc.expError {
				s.Require().Error(err)
				s.Require().NotNil(tc.wantErr)
				precompiletestutil.RequireExactError(s.T(), err, tc.wantErr())
				s.Require().Empty(bz)
			} else {
				s.Require().NoError(err)
				s.Require().NotEmpty(bz)
				tc.postCheck(bz)
			}
		})
	}
}
