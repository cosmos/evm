package keeper_test

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/erc20/types"
	"github.com/ethereum/go-ethereum/common"
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
		name       string
		malleate   func()
		expectPass bool
	}{
		{
			"pass",
			func() {
				// Set Token
				pair := types.NewTokenPair(erc20Addr, "coin", types.OWNER_MODULE)
				suite.network.App.Erc20Keeper.SetToken(ctx, pair)

				// Set Allowance
				err := suite.network.App.Erc20Keeper.SetAllowance(ctx, erc20Addr, owner, spender, value)
				suite.Require().NoError(err)
				expRes = value
			},
			true,
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			ctx = suite.network.GetContext()

			tc.malleate()
			res, err := suite.network.App.Erc20Keeper.GetAllowance(ctx, erc20Addr, owner, spender)
			if tc.expectPass {
				suite.Require().NoError(err)
				suite.Require().Equal(expRes, res)
			} else {
				suite.Require().Error(err)
				suite.Require().Equal(common.Big0, res)
			}
		})
	}
}
