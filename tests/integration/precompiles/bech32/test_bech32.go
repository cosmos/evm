package bech32

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm/precompiles/bech32"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *PrecompileTestSuite) TestNewPrecompile() {
	testCases := []struct {
		name        string
		baseGas     uint64
		expPass     bool
		errContains string
	}{
		{
			"fail - new precompile with baseGas == 0",
			0,
			false,
			"baseGas cannot be zero",
		},
		{
			"success - new precompile with baseGas > 0",
			10,
			true,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// setup basic test suite
			s.SetupTest()
			p, err := bech32.NewPrecompile(tc.baseGas)
			if tc.expPass {
				s.Require().NoError(err)
				s.Require().NotNil(p)
				s.Require().Equal(tc.baseGas, p.RequiredGas([]byte{}))
			} else {
				s.Require().Error(err)
				s.Require().Nil(p)
				s.Require().Contains(err.Error(), tc.errContains)
			}
		})
	}
}

// TestRun tests the precompile's Run method.
func (s *PrecompileTestSuite) TestRun() {
	contract := vm.NewPrecompile(
		s.keyring.GetAddr(0),
		s.precompile.Address(),
		uint256.NewInt(0),
		uint64(1000000),
	)

	testCases := []struct {
		name        string
		malleate    func() *vm.Contract
		postCheck   func(data []byte)
		expPass     bool
		errContains string
	}{
		{
			"fail - invalid method",
			func() *vm.Contract {
				contract.Input = []byte("invalid")
				return contract
			},
			func([]byte) {},
			false,
			"no method with id",
		},
		{
			"fail - error during unpack",
			func() *vm.Contract {
				// only pass the method ID to the input
				contract.Input = bech32.HexToBech32Selector[:]
				return contract
			},
			func([]byte) {},
			false,
			"unexpected EOF",
		},
		{
			"fail - HexToBech32 method error",
			func() *vm.Contract {
				call := bech32.HexToBech32Call{
					Addr:   s.keyring.GetAddr(0),
					Prefix: "",
				}
				input, err := call.EncodeWithSelector()
				s.Require().NoError(err, "failed to encode input")

				// only pass the method ID to the input
				contract.Input = input
				return contract
			},
			func([]byte) {},
			false,
			"invalid bech32 human readable prefix (HRP)",
		},
		{
			"pass - hex to bech32 account (cosmos)",
			func() *vm.Contract {
				call := bech32.NewHexToBech32Call(
					s.keyring.GetAddr(0),
					sdk.GetConfig().GetBech32AccountAddrPrefix(),
        )
				input, err := call.EncodeWithSelector()
				s.Require().NoError(err, "failed to encode input")
				contract.Input = input
				return contract
			},
			func(data []byte) {
				var ret bech32.HexToBech32Return
				_, err := ret.Decode(data)
				s.Require().NoError(err, "failed to unpack output")
				s.Require().Equal(s.keyring.GetAccAddr(0).String(), ret.Bech32Address)
			},
			true,
			"",
		},
		{
			"pass - hex to bech32 validator operator (cosmosvaloper)",
			func() *vm.Contract {
				valAddrCodec := s.network.App.GetStakingKeeper().ValidatorAddressCodec()
				valAddrBz, err := valAddrCodec.StringToBytes(s.network.GetValidators()[0].GetOperator())
				s.Require().NoError(err, "failed to convert string to bytes")
				call := bech32.NewHexToBech32Call(
					common.BytesToAddress(valAddrBz),
					sdk.GetConfig().GetBech32ValidatorAddrPrefix()
        )
				input, err := call.EncodeWithSelector()
				s.Require().NoError(err, "failed to encode input")
				contract.Input = input
				return contract
			},
			func(data []byte) {
				var ret bech32.HexToBech32Return
				_, err := ret.Decode(data)
				s.Require().NoError(err, "failed to unpack output")
				s.Require().Equal(s.network.GetValidators()[0].OperatorAddress, ret.Bech32Address)
			},
			true,
			"",
		},
		{
			"pass - hex to bech32 consensus address (cosmosvalcons)",
			func() *vm.Contract {
				call := bech32.HexToBech32Call{
					s.keyring.GetAddr(0),
					sdk.GetConfig().GetBech32ConsensusAddrPrefix(),
				}
				input, err := call.EncodeWithSelector()
				s.Require().NoError(err, "failed to encode input")
				contract.Input = input
				return contract
			},
			func(data []byte) {
				var ret bech32.HexToBech32Return
				_, err := ret.Decode(data)
				s.Require().NoError(err, "failed to unpack output")
				s.Require().Equal(sdk.ConsAddress(s.keyring.GetAddr(0).Bytes()).String(), ret.Bech32Address)
			},
			true,
			"",
		},
		{
			"pass - bech32 to hex account address",
			func() *vm.Contract {
				call := bech32.Bech32ToHexCall{
					Bech32Address: s.keyring.GetAccAddr(0).String(),
				}
				input, err := call.EncodeWithSelector()
				s.Require().NoError(err, "failed to encode input")
				contract.Input = input
				return contract
			},
			func(data []byte) {
				var ret bech32.Bech32ToHexReturn
				_, err := ret.Decode(data)
				s.Require().NoError(err, "failed to unpack output")
				s.Require().Equal(s.keyring.GetAddr(0), ret.Addr)
			},
			true,
			"",
		},
		{
			"pass - bech32 to hex validator address",
			func() *vm.Contract {
				call := bech32.Bech32ToHexCall{
					Bech32Address: s.network.GetValidators()[0].OperatorAddress,
				}
				input, err := call.EncodeWithSelector()
				s.Require().NoError(err, "failed to encode input")
				contract.Input = input
				return contract
			},
			func(data []byte) {
				valAddrCodec := s.network.App.GetStakingKeeper().ValidatorAddressCodec()
				valAddrBz, err := valAddrCodec.StringToBytes(s.network.GetValidators()[0].GetOperator())
				s.Require().NoError(err, "failed to convert string to bytes")

				var ret bech32.Bech32ToHexReturn
				_, err = ret.Decode(data)
				s.Require().NoError(err, "failed to unpack output")
				s.Require().Equal(common.BytesToAddress(valAddrBz), ret.Addr)
			},
			true,
			"",
		},
		{
			"pass - bech32 to hex consensus address",
			func() *vm.Contract {
				call := bech32.Bech32ToHexCall{
					Bech32Address: sdk.ConsAddress(s.keyring.GetAddr(0).Bytes()).String(),
				}
				input, err := call.EncodeWithSelector()
				s.Require().NoError(err, "failed to encode input")
				contract.Input = input
				return contract
			},
			func(data []byte) {
				var ret bech32.Bech32ToHexReturn
				_, err := ret.Decode(data)
				s.Require().NoError(err, "failed to unpack output")
				s.Require().Equal(s.keyring.GetAddr(0), ret.Addr)
			},
			true,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// setup basic test suite
			s.SetupTest()

			// malleate testcase
			contract := tc.malleate()

			// Run precompiled contract

			// NOTE: we can ignore the EVM and readonly args since it's a stateless
			// precompiled contract
			bz, err := s.precompile.Run(nil, contract, true)

			// Check results
			if tc.expPass {
				s.Require().NoError(err, "expected no error when running the precompile")
				s.Require().NotNil(bz, "expected returned bytes not to be nil")
				tc.postCheck(bz)
			} else {
				s.Require().Error(err, "expected error to be returned when running the precompile")
				s.Require().Nil(bz, "expected returned bytes to be nil")
				s.Require().ErrorContains(err, tc.errContains)
			}
		})
	}
}
