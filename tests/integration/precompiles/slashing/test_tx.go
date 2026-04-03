package slashing

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/slashing"
	"github.com/cosmos/evm/precompiles/testutil"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *PrecompileTestSuite) TestUnjail() {
	method := s.precompile.Methods[slashing.UnjailMethod]
	testCases := []struct {
		name      string
		malleate  func() []interface{}
		postCheck func()
		gas       uint64
		expError  bool
		wantErrFn func() error
	}{
		{
			"fail - empty input args",
			func() []interface{} {
				return []interface{}{}
			},
			func() {},
			200000,
			true,
			func() error {
				return cmn.NewRevertWithSolidityError(slashing.ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(1), big.NewInt(0))
			},
		},
		{
			"fail - invalid validator address",
			func() []interface{} {
				return []interface{}{
					"",
				}
			},
			func() {},
			200000,
			true,
			func() error {
				return cmn.NewRevertWithSolidityError(slashing.ABI, cmn.SolidityErrInvalidAddress, "")
			},
		},
		{
			"fail - msg.sender address does not match the validator address (empty address)",
			func() []interface{} {
				return []interface{}{
					common.Address{},
				}
			},
			func() {},
			200000,
			true,
			func() error {
				return cmn.NewRevertWithSolidityError(
					slashing.ABI,
					cmn.SolidityErrRequesterIsNotMsgSender,
					s.keyring.GetAddr(0),
					common.Address{},
				)
			},
		},
		{
			"fail - msg.sender address does not match the validator address",
			func() []interface{} {
				// any non-caller address is fine; keep deterministic for exact error matching
				return []interface{}{
					common.HexToAddress("0x0000000000000000000000000000000000000001"),
				}
			},
			func() {},
			200000,
			true,
			func() error {
				return cmn.NewRevertWithSolidityError(
					slashing.ABI,
					cmn.SolidityErrRequesterIsNotMsgSender,
					s.keyring.GetAddr(0),
					common.HexToAddress("0x0000000000000000000000000000000000000001"),
				)
			},
		},
		{
			"fail - validator not jailed",
			func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(0),
				}
			},
			func() {},
			200000,
			true,
			func() error {
				return cmn.NewRevertWithSolidityError(
					slashing.ABI,
					cmn.SolidityErrMsgServerFailed,
					slashing.UnjailMethod,
					"validator not jailed; cannot be unjailed",
				)
			},
		},
		{
			"success - validator unjailed",
			func() []interface{} {
				validator, err := s.network.App.GetStakingKeeper().GetValidator(s.network.GetContext(), sdk.ValAddress(s.keyring.GetAccAddr(0)))
				s.Require().NoError(err)

				valConsAddr, err := validator.GetConsAddr()
				s.Require().NoError(err)
				err = s.network.App.GetSlashingKeeper().Jail(
					s.network.GetContext(),
					valConsAddr,
				)
				s.Require().NoError(err)

				validatorAfterJail, err := s.network.App.GetStakingKeeper().GetValidator(s.network.GetContext(), sdk.ValAddress(s.keyring.GetAddr(0).Bytes()))
				s.Require().NoError(err)
				s.Require().True(validatorAfterJail.IsJailed())

				return []interface{}{
					s.keyring.GetAddr(0),
				}
			},
			func() {
				validatorAfterUnjail, err := s.network.App.GetStakingKeeper().GetValidator(s.network.GetContext(), sdk.ValAddress(s.keyring.GetAddr(0).Bytes()))
				s.Require().NoError(err)
				s.Require().False(validatorAfterUnjail.IsJailed())
			},
			200000,
			false,
			nil,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			contract, ctx := testutil.NewPrecompileContract(
				s.T(),
				s.network.GetContext(),
				s.keyring.GetAddr(0),
				s.precompile.Address(),
				tc.gas,
			)

			res, err := s.precompile.Unjail(ctx, &method, s.network.GetStateDB(), contract, tc.malleate())

			if tc.expError {
				s.Require().Error(err)
				s.Require().NotNil(tc.wantErrFn)
				testutil.RequireExactError(s.T(), err, tc.wantErrFn())
			} else {
				s.Require().NoError(err)
				s.Require().Equal(cmn.TrueValue, res)
				tc.postCheck()
			}
		})
	}
}
