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
	method := s.precompile.Methods[slashing.UnjailMethod]
	testCases := []struct {
		name        string
		malleate    func() []interface{}
		postCheck   func()
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"fail - empty input args",
			func() []interface{} {
				return []interface{}{}
			},
			func() {},
			200000,
			true,
			fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 1, 0),
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
			"invalid validator hex address",
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
			"does not match the requester address",
		},
		{
			"fail - msg.sender address does not match the validator address",
			func() []interface{} {
				return []interface{}{
					utiltx.GenerateAddress(),
				}
			},
			func() {},
			200000,
			true,
			"does not match the requester address",
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
			"validator not jailed",
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

			res, err := s.precompile.Unjail(ctx, &method, s.network.GetStateDB(), contract, tc.malleate())

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
