package slashing

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/slashing"
	"github.com/cosmos/evm/precompiles/testutil"
	utiltx "github.com/cosmos/evm/testutil/tx"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *PrecompileTestSuite) TestUnjail() {
	testCases := []struct {
		name        string
		malleate    func() slashing.UnjailCall
		postCheck   func()
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"fail - empty input args",
			func() slashing.UnjailCall {
				return slashing.UnjailCall{}
			},
			func() {},
			200000,
			true,
			fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 1, 0),
		},
		{
			"fail - invalid validator address",
			func() slashing.UnjailCall {
				return slashing.UnjailCall{
					ValidatorAddress: common.Address{},
				}
			},
			func() {},
			200000,
			true,
			"invalid validator hex address",
		},
		{
			"fail - msg.sender address does not match the validator address (empty address)",
			func() slashing.UnjailCall {
				return slashing.UnjailCall{
					ValidatorAddress: common.Address{},
				}
			},
			func() {},
			200000,
			true,
			"does not match the requester address",
		},
		{
			"fail - msg.sender address does not match the validator address",
			func() slashing.UnjailCall {
				return slashing.UnjailCall{
					ValidatorAddress: utiltx.GenerateAddress(),
				}
			},
			func() {},
			200000,
			true,
			"does not match the requester address",
		},
		{
			"fail - validator not jailed",
			func() slashing.UnjailCall {
				return slashing.UnjailCall{
					ValidatorAddress: s.keyring.GetAddr(0),
				}
			},
			func() {},
			200000,
			true,
			"validator not jailed",
		},
		{
			"success - validator unjailed",
			func() slashing.UnjailCall {
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

				return slashing.UnjailCall{
					ValidatorAddress: s.keyring.GetAddr(0),
				}
			},
			func() {
				validatorAfterUnjail, err := s.network.App.GetStakingKeeper().GetValidator(s.network.GetContext(), sdk.ValAddress(s.keyring.GetAddr(0).Bytes()))
				s.Require().NoError(err)
				s.Require().False(validatorAfterUnjail.IsJailed())
			},
			200000,
			false,
			"",
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

			call := tc.malleate()
			res, err := s.precompile.Unjail(ctx, &call, s.network.GetStateDB(), contract)

			if tc.expError {
				s.Require().ErrorContains(err, tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().Equal(cmn.TrueValue, res)
				tc.postCheck()
			}
		})
	}
}
