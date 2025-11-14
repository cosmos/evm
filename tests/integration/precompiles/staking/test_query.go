package staking

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/staking"
	testutiltx "github.com/cosmos/evm/testutil/tx"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (s *PrecompileTestSuite) TestDelegation() {
	testCases := []struct {
		name        string
		malleate    func(operatorAddress string) *staking.DelegationCall
		postCheck   func(out staking.DelegationReturn)
		gas         uint64
		expErr      bool
		errContains string
	}{
		{
			"fail - invalid operator address",
			func(string) *staking.DelegationCall {
				return staking.NewDelegationCall(
					s.keyring.GetAddr(0),
					"invalid",
				)
			},
			func(staking.DelegationReturn) {},
			100000,
			true,
			"invalid: unknown address",
		},
		{
			"success - empty delegation",
			func(operatorAddress string) *staking.DelegationCall {
				addr, _ := testutiltx.NewAddrKey()
				return staking.NewDelegationCall(
					addr,
					operatorAddress,
				)
			},
			func(out staking.DelegationReturn) {
				s.Require().Equal(out.Shares.Int64(), common.U2560.ToBig().Int64())
			},
			100000,
			false,
			"",
		},
		{
			"success",
			func(operatorAddress string) *staking.DelegationCall {
				return staking.NewDelegationCall(
					s.keyring.GetAddr(0),
					operatorAddress,
				)
			},
			func(out staking.DelegationReturn) {
				s.Require().Equal(out.Shares, big.NewInt(1e18))
			},
			100000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			out, err := s.precompile.Delegation(s.network.GetContext(), *tc.malleate(s.network.GetValidators()[0].OperatorAddress))

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				tc.postCheck(*out)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestUnbondingDelegation() {
	testCases := []struct {
		name        string
		malleate    func(operatorAddress string) *staking.UnbondingDelegationCall
		postCheck   func(staking.UnbondingDelegationReturn)
		gas         uint64
		expErr      bool
		errContains string
	}{
		{
			"success - no unbonding delegation found",
			func(operatorAddress string) *staking.UnbondingDelegationCall {
				addr, _ := testutiltx.NewAddrKey()
				return staking.NewUnbondingDelegationCall(
					addr,
					operatorAddress,
				)
			},
			func(ubdOut staking.UnbondingDelegationReturn) {
				s.Require().Len(ubdOut.UnbondingDelegation.Entries, 0)
			},
			100000,
			false,
			"",
		},
		{
			"success",
			func(operatorAddress string) *staking.UnbondingDelegationCall {
				return staking.NewUnbondingDelegationCall(
					s.keyring.GetAddr(0),
					operatorAddress,
				)
			},
			func(ubdOut staking.UnbondingDelegationReturn) {
				s.Require().Len(ubdOut.UnbondingDelegation.Entries, 1)
				s.Require().Equal(ubdOut.UnbondingDelegation.Entries[0].CreationHeight, s.network.GetContext().BlockHeight())
				s.Require().Equal(ubdOut.UnbondingDelegation.Entries[0].Balance, big.NewInt(1e18))
			},
			100000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset

			valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
			s.Require().NoError(err)
			_, _, err = s.network.App.GetStakingKeeper().Undelegate(s.network.GetContext(), s.keyring.GetAddr(0).Bytes(), valAddr, math.LegacyNewDec(1))
			s.Require().NoError(err)

			out, err := s.precompile.UnbondingDelegation(s.network.GetContext(), *tc.malleate(s.network.GetValidators()[0].OperatorAddress))

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(out)
				tc.postCheck(*out)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestValidator() {
	testCases := []struct {
		name        string
		malleate    func(operatorAddress common.Address) *staking.ValidatorCall
		postCheck   func(staking.ValidatorReturn)
		gas         uint64
		expErr      bool
		errContains string
	}{
		{
			"success",
			staking.NewValidatorCall,
			func(valOut staking.ValidatorReturn) {
				operatorAddress, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
				s.Require().NoError(err)

				s.Require().Equal(valOut.Validator, common.BytesToAddress(operatorAddress.Bytes()))
			},
			100000,
			false,
			"",
		},
		{
			name: "success - empty validator",
			malleate: func(_ common.Address) *staking.ValidatorCall {
				newAddr, _ := testutiltx.NewAccAddressAndKey()
				newValAddr := sdk.ValAddress(newAddr)
				return staking.NewValidatorCall(
					common.BytesToAddress(newValAddr.Bytes()),
				)
			},
			postCheck: func(valOut staking.ValidatorReturn) {
				s.Require().Equal(valOut.Validator.OperatorAddress, "")
				s.Require().Equal(valOut.Validator.Status, uint8(0))
			},
			gas: 100000,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			operatorAddress, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
			s.Require().NoError(err)

			out, err := s.precompile.Validator(s.network.GetContext(), *tc.malleate(common.BytesToAddress(operatorAddress.Bytes())))

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(out)
				tc.postCheck(*out)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestValidators() {
	testCases := []struct {
		name        string
		malleate    func() *staking.ValidatorsCall
		postCheck   func(staking.ValidatorsReturn)
		gas         uint64
		expErr      bool
		errContains string
	}{
		{
			"success - bonded status & pagination w/countTotal",
			func() *staking.ValidatorsCall {
				return staking.NewValidatorsCall(
					stakingtypes.Bonded.String(),
					cmn.PageRequest{
						Limit:      1,
						CountTotal: true,
					},
				)
			},
			func(valOut staking.ValidatorsReturn) {
				const expLen = 1
				s.Require().Len(valOut.Validators, expLen)
				// passed CountTotal = true
				s.Require().Equal(len(s.network.GetValidators()), int(valOut.PageResponse.Total)) //nolint:gosec
				s.Require().NotEmpty(valOut.PageResponse.NextKey)
				s.assertValidatorsResponse(valOut.Validators, expLen)
			},
			100000,
			false,
			"",
		},
		{
			"success - bonded status & pagination w/countTotal & key is []byte{0}",
			func() *staking.ValidatorsCall {
				return staking.NewValidatorsCall(
					stakingtypes.Bonded.String(),
					cmn.PageRequest{
						Key:        []byte{0},
						Limit:      1,
						CountTotal: true,
					},
				)
			},
			func(valOut staking.ValidatorsReturn) {
				const expLen = 1

				s.Require().Len(valOut.Validators, expLen)
				// passed CountTotal = true
				s.Require().Equal(len(s.network.GetValidators()), int(valOut.PageResponse.Total)) //nolint:gosec
				s.Require().NotEmpty(valOut.PageResponse.NextKey)
				s.assertValidatorsResponse(valOut.Validators, expLen)
			},
			100000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			out, err := s.precompile.Validators(s.network.GetContext(), *tc.malleate())

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(out)
				tc.postCheck(*out)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestRedelegation() {
	testCases := []struct {
		name        string
		malleate    func(srcOperatorAddr, destOperatorAddr string) *staking.RedelegationCall
		postCheck   func(staking.RedelegationOutput)
		gas         uint64
		expErr      bool
		errContains string
	}{
		{
			"fail - empty src validator addr",
			func(_, destOperatorAddr string) *staking.RedelegationCall {
				return staking.NewRedelegationCall(
					s.keyring.GetAddr(0),
					"",
					destOperatorAddr,
				)
			},
			func(staking.RedelegationOutput) {},
			100000,
			true,
			"empty address string is not allowed",
		},
		{
			"fail - empty destination addr",
			func(srcOperatorAddr, _ string) *staking.RedelegationCall {
				return staking.NewRedelegationCall(
					s.keyring.GetAddr(0),
					srcOperatorAddr,
					"",
				)
			},
			func(staking.RedelegationOutput) {},
			100000,
			true,
			"empty address string is not allowed",
		},
		{
			"success",
			func(srcOperatorAddr, destOperatorAddr string) *staking.RedelegationCall {
				return staking.NewRedelegationCall(
					s.keyring.GetAddr(0),
					srcOperatorAddr,
					destOperatorAddr,
				)
			},
			func(redOut staking.RedelegationOutput) {
				s.Require().Len(redOut.Entries, 1)
				s.Require().Equal(redOut.Entries[0].CreationHeight, s.network.GetContext().BlockHeight())
				s.Require().Equal(redOut.Entries[0].SharesDst, big.NewInt(1e18))
			},
			100000,
			false,
			"",
		},
		{
			name: "success - no redelegation found",
			malleate: func(srcOperatorAddr, _ string) *staking.RedelegationCall {
				nonExistentOperator := sdk.ValAddress([]byte("non-existent-operator"))
				return staking.NewRedelegationCall(
					s.keyring.GetAddr(0),
					srcOperatorAddr,
					nonExistentOperator.String(),
				)
			},
			postCheck: func(redOut staking.RedelegationOutput) {
				s.Require().Len(redOut.Entries, 0)
			},
			gas: 100000,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			contract := vm.NewContract(s.keyring.GetAddr(0), s.precompile.Address(), uint256.NewInt(0), tc.gas, nil)

			delegationArgs := staking.NewRedelegateCall(
				s.keyring.GetAddr(0),
				s.network.GetValidators()[0].OperatorAddress,
				s.network.GetValidators()[1].OperatorAddress,
				big.NewInt(1e18),
			)

			_, err := s.precompile.Redelegate(s.network.GetContext(), *delegationArgs, s.network.GetStateDB(), contract)
			s.Require().NoError(err)

			out, err := s.precompile.Redelegation(s.network.GetContext(), *tc.malleate(s.network.GetValidators()[0].OperatorAddress, s.network.GetValidators()[1].OperatorAddress))

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(out)
				tc.postCheck(out.Redelegation)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestRedelegations() {
	var (
		delAmt                 = big.NewInt(3e17)
		redelTotalCount uint64 = 2
	)

	testCases := []struct {
		name        string
		malleate    func() *staking.RedelegationsCall
		postCheck   func(staking.RedelegationsReturn)
		gas         uint64
		expErr      bool
		errContains string
	}{
		{
			"fail - invalid query | all empty args ",
			func() *staking.RedelegationsCall {
				return staking.NewRedelegationsCall(
					common.Address{},
					"",
					"",
					cmn.PageRequest{},
				)
			},
			func(out staking.RedelegationsReturn) {},
			100000,
			true,
			"invalid query. Need to specify at least a source validator address or delegator address",
		},
		{
			"fail - invalid query | only destination validator address",
			func() *staking.RedelegationsCall {
				return staking.NewRedelegationsCall(
					common.Address{},
					"",
					s.network.GetValidators()[1].OperatorAddress,
					cmn.PageRequest{},
				)
			},
			func(out staking.RedelegationsReturn) {},
			100000,
			true,
			"invalid query. Need to specify at least a source validator address or delegator address",
		},
		{
			"success - specified delegator, source & destination",
			func() *staking.RedelegationsCall {
				return staking.NewRedelegationsCall(
					s.keyring.GetAddr(0),
					s.network.GetValidators()[0].OperatorAddress,
					s.network.GetValidators()[1].OperatorAddress,
					cmn.PageRequest{},
				)
			},
			func(out staking.RedelegationsReturn) {
				s.assertRedelegationsOutput(out, 0, delAmt, s.network.GetContext().BlockHeight(), false)
			},
			100000,
			false,
			"",
		},
		{
			"success - specifying only source w/pagination",
			func() *staking.RedelegationsCall {
				return staking.NewRedelegationsCall(
					common.Address{},
					s.network.GetValidators()[0].OperatorAddress,
					"",
					cmn.PageRequest{
						Limit:      1,
						CountTotal: true,
					},
				)
			},
			func(out staking.RedelegationsReturn) {
				s.assertRedelegationsOutput(out, redelTotalCount, delAmt, s.network.GetContext().BlockHeight(), true)
			},
			100000,
			false,
			"",
		},
		{
			"success - get all existing redelegations for a delegator w/pagination",
			func() *staking.RedelegationsCall {
				return staking.NewRedelegationsCall(
					s.keyring.GetAddr(0),
					"",
					"",
					cmn.PageRequest{
						Limit:      1,
						CountTotal: true,
					},
				)
			},
			func(out staking.RedelegationsReturn) {
				s.assertRedelegationsOutput(out, redelTotalCount, delAmt, s.network.GetContext().BlockHeight(), true)
			},
			100000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			err := s.setupRedelegations(s.network.GetContext(), delAmt)
			s.Require().NoError(err)

			// query redelegations
			out, err := s.precompile.Redelegations(s.network.GetContext(), *tc.malleate())

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				tc.postCheck(*out)
			}
		})
	}
}
