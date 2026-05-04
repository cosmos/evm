package keeper_test

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/erc20/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

func (suite *KeeperTestSuite) TestGetAllowance() {
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
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
				expRes = common.Big0
			},
			true,
			"",
		},
		{
			"pass - allowance does not exist",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
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
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)

				// Set Allowance
				err = suite.network.App.Erc20Keeper.SetAllowance(ctx, erc20Addr, owner, spender, value)
				suite.Require().NoError(err)
				expRes = value
			},
			true,
			"",
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()

			tc.malleate()

			// Get Allowance
			res, err := suite.network.App.Erc20Keeper.GetAllowance(ctx, erc20Addr, owner, spender)
			if tc.expectPass {
				suite.Require().NoError(err)
				suite.Require().Equal(expRes, res)
			} else {
				suite.Require().Error(err)
				suite.Require().ErrorContains(err, tc.errContains)
				suite.Require().Equal(common.Big0, res)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestSetAllowance() {
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
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
			},
			false,
			types.ErrERC20TokenPairDisabled.Error(),
		},
		{
			"fail - zero owner address",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
				owner = common.HexToAddress("0x0")
			},
			false,
			errortypes.ErrInvalidAddress.Error(),
		},
		{
			"fail - zero spender address",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
				spender = common.HexToAddress("0x0")
			},
			false,
			errortypes.ErrInvalidAddress.Error(),
		},
		{
			"fail - negative value",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
				value = big.NewInt(-100)
			},
			false,
			types.ErrInvalidAllowance.Error(),
		},
		{
			"pass - zero value",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
				value = big.NewInt(0)
			},
			true,
			"",
		},
		{
			"pass - positive value",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
				value = big.NewInt(100)
			},
			true,
			"",
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()

			initArgs()
			tc.malleate()

			// Set Allowance
			err := suite.network.App.Erc20Keeper.SetAllowance(ctx, erc20Addr, owner, spender, value)
			if tc.expectPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
				suite.Require().ErrorContains(err, tc.errContains)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestUnsafeSetAllowance() {
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
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
			},
			true,
			"",
		},
		{
			"fail - zero owner address",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
				owner = common.HexToAddress("0x0")
			},
			false,
			errortypes.ErrInvalidAddress.Error(),
		},
		{
			"fail - zero spender address",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
				spender = common.HexToAddress("0x0")
			},
			false,
			errortypes.ErrInvalidAddress.Error(),
		},
		{
			"fail - negative value",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
				value = big.NewInt(-100)
			},
			false,
			types.ErrInvalidAllowance.Error(),
		},
		{
			"pass - zero value",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
				value = big.NewInt(0)
			},
			true,
			"",
		},
		{
			"pass - positive value",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
				value = big.NewInt(100)
			},
			true,
			"",
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()

			initArgs()
			tc.malleate()

			// Set Allowance
			err := suite.network.App.Erc20Keeper.UnsafeSetAllowance(ctx, erc20Addr, owner, spender, value)
			if tc.expectPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
				suite.Require().ErrorContains(err, tc.errContains)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestDeleteAllowance() {
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
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
			},
			false,
			types.ErrERC20TokenPairDisabled.Error(),
		},
		{
			"fail - zero owner address",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
				owner = common.HexToAddress("0x0")
			},
			false,
			errortypes.ErrInvalidAddress.Error(),
		},
		{
			"fail - zero spender address",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
				spender = common.HexToAddress("0x0")
			},
			false,
			errortypes.ErrInvalidAddress.Error(),
		},
		{
			"pass - for non-existing allowance",
			func() {
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)
			},
			true,
			"",
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()

			initArgs()
			tc.malleate()

			// Delete Allowance
			err := suite.network.App.Erc20Keeper.DeleteAllowance(ctx, erc20Addr, owner, spender)
			if tc.expectPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
				suite.Require().ErrorContains(err, tc.errContains)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestGetAllowances() {
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
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)

				err = suite.network.App.Erc20Keeper.SetAllowance(ctx, erc20Addr, owner, spender, value)
				suite.Require().NoError(err)

				// Delete TokenPair
				suite.network.App.Erc20Keeper.DeleteTokenPair(ctx, pair)

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
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)

				err = suite.network.App.Erc20Keeper.SetAllowance(ctx, erc20Addr, owner, spender, value)
				suite.Require().NoError(err)

				pair.Enabled = false
				suite.network.App.Erc20Keeper.SetTokenPair(ctx, pair)
				pairID := suite.network.App.Erc20Keeper.GetDenomMap(ctx, pair.Denom)
				pair, ok := suite.network.App.Erc20Keeper.GetTokenPair(ctx, pairID)
				suite.Require().True(ok)
				suite.Require().False(pair.Enabled)

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
				err := suite.network.App.Erc20Keeper.SetToken(ctx, pair)
				suite.Require().NoError(err)

				err = suite.network.App.Erc20Keeper.SetAllowance(ctx, erc20Addr, owner, spender, value)
				suite.Require().NoError(err)

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
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()

			tc.malleate()

			// Get Allowance
			res := suite.network.App.Erc20Keeper.GetAllowances(ctx)
			suite.Require().Equal(expRes, res)
		})
	}
}
