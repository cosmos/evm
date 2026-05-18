package erc20

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/erc20/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

func (s *KeeperTestSuite) TestGetAllowance() {
	var (
		ctx       sdk.Context
		expRes    *big.Int
		erc20Addr = utiltx.GenerateAddress()
		owner     = utiltx.GenerateAddress()
		spender   = utiltx.GenerateAddress()
		value     = big.NewInt(100)
	)

	testCases := []struct {
		name        string
		malleate    func()
		expectPass  bool
		errContains string
	}{
		{
			"fail - token pair does not exist",
			func() {
				expRes = common.Big0
			},
			true,
			"",
		},
		{
			"pass - token pair is disabled",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				pair.Enabled = false
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
				expRes = common.Big0
			},
			true,
			"",
		},
		{
			"pass - allowance does not exist",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
				expRes = common.Big0
			},
			true,
			"",
		},
		{
			"pass",
			func() {
				// Set TokenPair
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)

				// Set Allowance
				err = s.network.App.GetErc20Keeper().SetAllowance(ctx, erc20Addr, owner, spender, value)
				s.Require().NoError(err)
				expRes = value
			},
			true,
			"",
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()

			tc.malleate()

			// Get Allowance
			res, err := s.network.App.GetErc20Keeper().GetAllowance(ctx, erc20Addr, owner, spender)
			if tc.expectPass {
				s.Require().NoError(err)
				s.Require().Equal(expRes, res)
			} else {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.errContains)
				s.Require().Equal(common.Big0, res)
			}
		})
	}
}

func (s *KeeperTestSuite) TestSetAllowance() {
	var (
		ctx       sdk.Context
		erc20Addr common.Address
		owner     common.Address
		spender   common.Address
		value     *big.Int

		initArgs = func() {
			erc20Addr = utiltx.GenerateAddress()
			owner = utiltx.GenerateAddress()
			spender = utiltx.GenerateAddress()
			value = big.NewInt(100)
		}
	)

	testCases := []struct {
		name        string
		malleate    func()
		expectPass  bool
		errContains string
	}{
		{
			"fail - no token pair exists",
			func() {},
			false,
			types.ErrTokenPairNotFound.Error(),
		},
		{
			"fail - token pair is disabled",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				pair.Enabled = false
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
			},
			false,
			types.ErrERC20TokenPairDisabled.Error(),
		},
		{
			"fail - zero owner address",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
				owner = common.HexToAddress("0x0")
			},
			false,
			errortypes.ErrInvalidAddress.Error(),
		},
		{
			"fail - zero spender address",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
				spender = common.HexToAddress("0x0")
			},
			false,
			errortypes.ErrInvalidAddress.Error(),
		},
		{
			"fail - negative value",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
				value = big.NewInt(-100)
			},
			false,
			types.ErrInvalidAllowance.Error(),
		},
		{
			"pass - zero value",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
				value = big.NewInt(0)
			},
			true,
			"",
		},
		{
			"pass - positive value",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
				value = big.NewInt(100)
			},
			true,
			"",
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()

			initArgs()
			tc.malleate()

			// Set Allowance
			err := s.network.App.GetErc20Keeper().SetAllowance(ctx, erc20Addr, owner, spender, value)
			if tc.expectPass {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.errContains)
			}
		})
	}
}

func (s *KeeperTestSuite) TestUnsafeSetAllowance() {
	var (
		ctx       sdk.Context
		erc20Addr common.Address
		owner     common.Address
		spender   common.Address
		value     *big.Int

		initArgs = func() {
			erc20Addr = utiltx.GenerateAddress()
			owner = utiltx.GenerateAddress()
			spender = utiltx.GenerateAddress()
			value = big.NewInt(100)
		}
	)

	testCases := []struct {
		name        string
		malleate    func()
		expectPass  bool
		errContains string
	}{
		{
			"fail - no token pair exists",
			func() {},
			false,
			types.ErrTokenPairNotFound.Error(),
		},
		{
			"pass - token pair is disabled",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				pair.Enabled = false
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
			},
			true,
			"",
		},
		{
			"fail - zero owner address",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
				owner = common.HexToAddress("0x0")
			},
			false,
			errortypes.ErrInvalidAddress.Error(),
		},
		{
			"fail - zero spender address",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
				spender = common.HexToAddress("0x0")
			},
			false,
			errortypes.ErrInvalidAddress.Error(),
		},
		{
			"fail - negative value",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
				value = big.NewInt(-100)
			},
			false,
			types.ErrInvalidAllowance.Error(),
		},
		{
			"pass - zero value",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
				value = big.NewInt(0)
			},
			true,
			"",
		},
		{
			"pass - positive value",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
				value = big.NewInt(100)
			},
			true,
			"",
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()

			initArgs()
			tc.malleate()

			// Set Allowance
			err := s.network.App.GetErc20Keeper().UnsafeSetAllowance(ctx, erc20Addr, owner, spender, value)
			if tc.expectPass {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.errContains)
			}
		})
	}
}

func (s *KeeperTestSuite) TestDeleteAllowance() {
	var (
		ctx       sdk.Context
		erc20Addr common.Address
		owner     common.Address
		spender   common.Address

		initArgs = func() {
			erc20Addr = utiltx.GenerateAddress()
			owner = utiltx.GenerateAddress()
			spender = utiltx.GenerateAddress()
		}
	)

	testCases := []struct {
		name        string
		malleate    func()
		expectPass  bool
		errContains string
	}{
		{
			"fail - no token pair exists",
			func() {},
			false,
			types.ErrTokenPairNotFound.Error(),
		},
		{
			"fail - token pair is disabled",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				pair.Enabled = false
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
			},
			false,
			types.ErrERC20TokenPairDisabled.Error(),
		},
		{
			"fail - zero owner address",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
				owner = common.HexToAddress("0x0")
			},
			false,
			errortypes.ErrInvalidAddress.Error(),
		},
		{
			"fail - zero spender address",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
				spender = common.HexToAddress("0x0")
			},
			false,
			errortypes.ErrInvalidAddress.Error(),
		},
		{
			"pass - for non-existing allowance",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)
			},
			true,
			"",
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()

			initArgs()
			tc.malleate()

			// Delete Allowance
			err := s.network.App.GetErc20Keeper().DeleteAllowance(ctx, erc20Addr, owner, spender)
			if tc.expectPass {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.errContains)
			}
		})
	}
}

func (s *KeeperTestSuite) TestGetAllowances() {
	var (
		ctx       sdk.Context
		expRes    []types.Allowance
		erc20Addr = utiltx.GenerateAddress()
		owner     = utiltx.GenerateAddress()
		spender   = utiltx.GenerateAddress()
		value     = big.NewInt(100)
	)

	testCases := []struct {
		name     string
		malleate func()
	}{
		{
			// NOTES: This case doesn’t actually occur in practice.
			// It is because, while Allowances exist only for the ERC20 precompile,
			// only ERC20 token that was initially deployed on EVM state can be deleted.
			"pass - even if token pair were deleted, allowances are deleted together and returns empty allowances",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)

				err = s.network.App.GetErc20Keeper().SetAllowance(ctx, erc20Addr, owner, spender, value)
				s.Require().NoError(err)

				// Delete TokenPair
				s.network.App.GetErc20Keeper().DeleteTokenPair(ctx, pair)

				expRes = []types.Allowance{}
			},
		},
		{
			// NOTES: GetAllowances() is only for genesis import & export.
			// Because disabled token pair can be enabled later,
			// when allowances related to disabled token pair should also be included in the exported state.
			"pass - even if token pair is disabled, return allowances",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)

				err = s.network.App.GetErc20Keeper().SetAllowance(ctx, erc20Addr, owner, spender, value)
				s.Require().NoError(err)

				pair.Enabled = false
				s.network.App.GetErc20Keeper().SetTokenPair(ctx, pair)
				pairID := s.network.App.GetErc20Keeper().GetDenomMap(ctx, pair.Denom)
				pair, ok := s.network.App.GetErc20Keeper().GetTokenPair(ctx, pairID)
				s.Require().True(ok)
				s.Require().False(pair.Enabled)

				expRes = []types.Allowance{
					{
						Erc20Address: erc20Addr.Hex(),
						Owner:        owner.Hex(),
						Spender:      spender.Hex(),
						Value:        math.NewIntFromBigInt(value),
					},
				}
			},
		},
		{
			"pass - no allowances",
			func() {
				expRes = []types.Allowance{}
			},
		},
		{
			"pass",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := s.network.App.GetErc20Keeper().SetToken(ctx, pair)
				s.Require().NoError(err)

				err = s.network.App.GetErc20Keeper().SetAllowance(ctx, erc20Addr, owner, spender, value)
				s.Require().NoError(err)

				expRes = []types.Allowance{
					{
						Erc20Address: erc20Addr.Hex(),
						Owner:        owner.Hex(),
						Spender:      spender.Hex(),
						Value:        math.NewIntFromBigInt(value),
					},
				}
			},
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()

			tc.malleate()

			// Get Allowance
			res := s.network.App.GetErc20Keeper().GetAllowances(ctx)
			s.Require().Equal(expRes, res)
		})
	}
}
