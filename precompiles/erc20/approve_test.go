package erc20_test

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/erc20"
	"github.com/cosmos/evm/precompiles/testutil"

	"cosmossdk.io/math"
)

//nolint:dupl // tests are not duplicate between the functions
func (s *PrecompileTestSuite) TestApprove() {
	method := s.precompile.Methods[erc20.ApproveMethod]
	amount := int64(100)

	testcases := []struct {
		name        string
		malleate    func() []interface{}
		postCheck   func()
		expPass     bool
		errContains string
	}{
		{
			name:        "fail - empty args",
			malleate:    func() []interface{} { return nil },
			errContains: "invalid number of arguments",
		},
		{
			name: "fail - invalid number of arguments",
			malleate: func() []interface{} {
				return []interface{}{
					1, 2, 3,
				}
			},
			errContains: "invalid number of arguments",
		},
		{
			name: "fail - invalid address",
			malleate: func() []interface{} {
				return []interface{}{
					"invalid address", big.NewInt(2),
				}
			},
			errContains: "invalid address",
		},
		{
			name: "fail - invalid amount",
			malleate: func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(1), "invalid amount",
				}
			},
			errContains: "invalid amount",
		},
		{
			name: "fail - negative amount",
			malleate: func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(1), big.NewInt(-1),
				}
			},
			errContains: erc20.ErrNegativeAmount.Error(),
		},
		{
			name: "fail - approve uint256 overflow",
			malleate: func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(1), new(big.Int).Add(abi.MaxUint256, common.Big1),
				}
			},
			errContains: "causes integer overflow",
		},
		{
			name: "pass - approve to zero with existing allowance only for other denominations",
			malleate: func() []interface{} {
				// NOTE: We are setting up an allowance for a different denomination
				// and then trying to approve an amount of zero for the token denomination
				s.setAllowance(
					s.precompile2.Address(),
					s.keyring.GetPrivKey(0),
					s.keyring.GetAddr(1),
					big.NewInt(1),
				)

				return []interface{}{
					s.keyring.GetAddr(1), common.Big0,
				}
			},
			expPass: true,
			postCheck: func() {
				// Check that the allowance is zero
				s.requireAllowance(
					s.precompile.Address(),
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(1),
					big.NewInt(0),
				)

				// Check that the allowance for the other denomination was not deleted
				s.requireAllowance(
					s.precompile2.Address(),
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(1),
					big.NewInt(1),
				)
			},
		},
		{
			name: "pass - approve without existing allowance",
			malleate: func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(1), big.NewInt(amount),
				}
			},
			expPass: true,
			postCheck: func() {
				s.requireAllowance(
					s.precompile.Address(),
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(1),
					big.NewInt(amount),
				)
			},
		},
		{
			name: "pass - approve with existing allowance",
			malleate: func() []interface{} {
				s.setAllowance(
					s.precompile.Address(),
					s.keyring.GetPrivKey(0),
					s.keyring.GetAddr(1),
					big.NewInt(1),
				)

				return []interface{}{
					s.keyring.GetAddr(1), big.NewInt(amount),
				}
			},
			expPass: true,
			postCheck: func() {
				s.requireAllowance(
					s.precompile.Address(),
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(1),
					big.NewInt(amount),
				)
			},
		},
		{
			name: "pass - approve with existing allowance in different denomination",
			malleate: func() []interface{} {
				s.setAllowance(
					s.precompile2.Address(),
					s.keyring.GetPrivKey(0),
					s.keyring.GetAddr(1),
					big.NewInt(1),
				)

				return []interface{}{
					s.keyring.GetAddr(1), big.NewInt(amount),
				}
			},
			expPass: true,
			postCheck: func() {
				// Check that the allowance is set to the new amount
				s.requireAllowance(
					s.precompile.Address(),
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(1),
					big.NewInt(amount),
				)

				// Check that the allowance for the other denomination was not deleted
				s.requireAllowance(
					s.precompile2.Address(),
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(1),
					big.NewInt(1),
				)
			},
		},
		{
			name: "pass - delete existing allowance",
			malleate: func() []interface{} {
				s.setAllowance(
					s.precompile.Address(),
					s.keyring.GetPrivKey(0),
					s.keyring.GetAddr(1),
					big.NewInt(1),
				)

				return []interface{}{
					s.keyring.GetAddr(1), common.Big0,
				}
			},
			expPass: true,
			postCheck: func() {
				s.requireAllowance(
					s.precompile.Address(),
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(1),
					common.Big0,
				)
			},
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			s.SetupTest()

			ctx := s.network.GetContext()

			var contract *vm.Contract
			contract, ctx = testutil.NewPrecompileContract(
				s.T(),
				ctx,
				s.keyring.GetAddr(0),
				s.precompile.Address(),
				200_000,
			)

			var args []interface{}
			if tc.malleate != nil {
				args = tc.malleate()
			}

			bz, err := s.precompile.Approve(
				ctx,
				contract,
				s.network.GetStateDB(),
				&method,
				args,
			)

			if tc.expPass {
				s.Require().NoError(err, "expected no error")
				s.Require().NotNil(bz, "expected non-nil bytes")
			} else {
				s.Require().Error(err, "expected error")
				s.Require().ErrorContains(err, tc.errContains, "expected different error message")
				s.Require().Empty(bz, "expected empty bytes")
			}

			if tc.postCheck != nil {
				tc.postCheck()
			}
		})
	}
}

//nolint:dupl // tests are not duplicate between the functions
func (s *PrecompileTestSuite) TestIncreaseAllowance() {
	method := s.precompile.Methods[erc20.IncreaseAllowanceMethod]
	amount := int64(100)
	increaseAmount := int64(200)

	testcases := []struct {
		name        string
		malleate    func() []interface{}
		postCheck   func()
		expPass     bool
		errContains string
	}{
		{
			name:        "fail - empty args",
			malleate:    func() []interface{} { return nil },
			errContains: "invalid number of arguments",
		},
		{
			name: "fail - invalid number of arguments",
			malleate: func() []interface{} {
				return []interface{}{
					1, 2, 3,
				}
			},
			errContains: "invalid number of arguments",
		},
		{
			name: "fail - invalid address",
			malleate: func() []interface{} {
				return []interface{}{
					"invalid address", big.NewInt(2),
				}
			},
			errContains: "invalid address",
		},
		{
			name: "fail - invalid amount",
			malleate: func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(1), "invalid amount",
				}
			},
			errContains: "invalid amount",
		},
		{
			name: "fail - negative amount",
			malleate: func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(1), big.NewInt(-1),
				}
			},
			errContains: erc20.ErrIncreaseNonPositiveValue.Error(),
		},
		{
			name: "pass - increase allowance without existing allowance",
			malleate: func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(1), big.NewInt(increaseAmount),
				}
			},
			expPass: true,
			postCheck: func() {
				s.requireAllowance(
					s.precompile.Address(),
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(1),
					big.NewInt(increaseAmount),
				)
			},
		},
		{
			name: "pass - increase allowance with existing allowance",
			malleate: func() []interface{} {
				s.setAllowance(
					s.precompile.Address(),
					s.keyring.GetPrivKey(0),
					s.keyring.GetAddr(1),
					big.NewInt(amount),
				)

				return []interface{}{
					s.keyring.GetAddr(1), big.NewInt(increaseAmount),
				}
			},
			expPass: true,
			postCheck: func() {
				s.requireAllowance(
					s.precompile.Address(),
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(1),
					big.NewInt(amount+increaseAmount),
				)
			},
		},
		{
			name: "fail - uint256 overflow when increasing allowance",
			malleate: func() []interface{} {
				// NOTE: We are setting up a allowance with the maximum uint256 value
				// and then trying to approve an amount that would overflow the uint256 value
				s.setAllowance(
					s.precompile.Address(),
					s.keyring.GetPrivKey(0),
					s.keyring.GetAddr(1),
					abi.MaxUint256,
				)

				return []interface{}{
					s.keyring.GetAddr(1), big.NewInt(amount),
				}
			},
			errContains: erc20.ConvertErrToERC20Error(errors.New(cmn.ErrIntegerOverflow)).Error(),
			postCheck: func() {
				s.requireAllowance(
					s.precompile.Address(),
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(1),
					math.NewIntFromBigInt(abi.MaxUint256).BigInt(),
				)
			},
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			s.SetupTest()

			ctx := s.network.GetContext()

			var contract *vm.Contract
			contract, ctx = testutil.NewPrecompileContract(
				s.T(),
				ctx,
				s.keyring.GetAddr(0),
				s.precompile.Address(),
				200_000,
			)

			var args []interface{}
			if tc.malleate != nil {
				args = tc.malleate()
			}

			bz, err := s.precompile.IncreaseAllowance(
				ctx,
				contract,
				s.network.GetStateDB(),
				&method,
				args,
			)

			if tc.expPass {
				s.Require().NoError(err, "expected no error")
				s.Require().NotNil(bz, "expected non-nil bytes")
			} else {
				s.Require().Error(err, "expected error")
				s.Require().ErrorContains(err, tc.errContains, "expected different error message")
				s.Require().Empty(bz, "expected empty bytes")
			}

			if tc.postCheck != nil {
				tc.postCheck()
			}
		})
	}
}

//nolint:dupl // tests are not duplicate between the functions
func (s *PrecompileTestSuite) TestDecreaseAllowance() {
	method := s.precompile.Methods[erc20.DecreaseAllowanceMethod]
	amount := int64(100)
	decreaseAmount := int64(50)

	testcases := []struct {
		name        string
		malleate    func() []interface{}
		postCheck   func()
		expPass     bool
		errContains string
	}{
		{
			name:        "fail - empty args",
			malleate:    func() []interface{} { return nil },
			errContains: "invalid number of arguments",
		},
		{
			name: "fail - invalid number of arguments",
			malleate: func() []interface{} {
				return []interface{}{
					1, 2, 3,
				}
			},
			errContains: "invalid number of arguments",
		},
		{
			name: "fail - invalid address",
			malleate: func() []interface{} {
				return []interface{}{
					"invalid address", big.NewInt(2),
				}
			},
			errContains: "invalid address",
		},
		{
			name: "fail - invalid amount",
			malleate: func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(1), "invalid amount",
				}
			},
			errContains: "invalid amount",
		},
		{
			name: "fail - negative amount",
			malleate: func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(1), big.NewInt(-1),
				}
			},
			errContains: erc20.ErrDecreaseNonPositiveValue.Error(),
		},
		{
			name: "fail - decrease allowance without existing allowance",
			malleate: func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(1), big.NewInt(decreaseAmount),
				}
			},
			errContains: "does not exist",
		},
		{
			name: "pass - decrease allowance with existing allowance",
			malleate: func() []interface{} {
				s.setAllowance(
					s.precompile.Address(),
					s.keyring.GetPrivKey(0),
					s.keyring.GetAddr(1),
					big.NewInt(amount),
				)

				return []interface{}{
					s.keyring.GetAddr(1), big.NewInt(decreaseAmount),
				}
			},
			expPass: true,
			postCheck: func() {
				s.requireAllowance(
					s.precompile.Address(),
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(1),
					big.NewInt(amount-decreaseAmount),
				)
			},
		},
		{
			name: "pass - decrease to zero and delete existing allowance",
			malleate: func() []interface{} {
				s.setAllowance(
					s.precompile.Address(),
					s.keyring.GetPrivKey(0),
					s.keyring.GetAddr(1),
					big.NewInt(amount),
				)

				return []interface{}{
					s.keyring.GetAddr(1), big.NewInt(amount),
				}
			},
			expPass: true,
			postCheck: func() {
				// Check that the allowance was deleted
				s.requireAllowance(
					s.precompile.Address(),
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(1),
					common.Big0,
				)
			},
		},
		{
			name: "pass - decrease allowance to zero for denom with existing allowance in other denominations",
			malleate: func() []interface{} {
				s.setAllowance(
					s.precompile.Address(),
					s.keyring.GetPrivKey(0),
					s.keyring.GetAddr(1),
					big.NewInt(amount),
				)

				s.setAllowance(
					s.precompile2.Address(),
					s.keyring.GetPrivKey(0),
					s.keyring.GetAddr(1),
					big.NewInt(amount),
				)

				return []interface{}{
					s.keyring.GetAddr(1), big.NewInt(amount),
				}
			},
			expPass: true,
			postCheck: func() {
				// Check that the allowance was deleted
				s.requireAllowance(
					s.precompile.Address(),
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(1),
					big.NewInt(0),
				)

				// Check that the allowance for the other denomination was deleted
				s.requireAllowance(
					s.precompile2.Address(),
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(1),
					big.NewInt(amount),
				)
			},
		},
		{
			name: "fail - decrease allowance with existing allowance but decreased amount too high",
			malleate: func() []interface{} {
				s.setAllowance(
					s.precompile.Address(),
					s.keyring.GetPrivKey(0),
					s.keyring.GetAddr(1),
					big.NewInt(amount),
				)

				return []interface{}{
					s.keyring.GetAddr(1), big.NewInt(amount + 1),
				}
			},
			errContains: erc20.ConvertErrToERC20Error(errors.New("subtracted value cannot be greater than existing allowance")).Error(),
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			s.SetupTest()

			ctx := s.network.GetContext()

			var contract *vm.Contract
			contract, ctx = testutil.NewPrecompileContract(
				s.T(),
				ctx,
				s.keyring.GetAddr(0),
				s.precompile.Address(),
				200_000,
			)

			var args []interface{}
			if tc.malleate != nil {
				args = tc.malleate()
			}

			bz, err := s.precompile.DecreaseAllowance(
				ctx,
				contract,
				s.network.GetStateDB(),
				&method,
				args,
			)

			if tc.expPass {
				s.Require().NoError(err, "expected no error")
				s.Require().NotNil(bz, "expected non-nil bytes")
			} else {
				s.Require().Error(err, "expected error")
				s.Require().ErrorContains(err, tc.errContains, "expected different error message")
				s.Require().Empty(bz, "expected empty bytes")
			}

			if tc.postCheck != nil {
				tc.postCheck()
			}
		})
	}
}
