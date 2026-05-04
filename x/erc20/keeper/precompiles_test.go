package keeper_test

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	testconstants "github.com/cosmos/evm/testutil/constants"
	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/erc20/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (suite *KeeperTestSuite) TestGetERC20PrecompileInstance() {
	var (
		ctx        sdk.Context
		tokenPairs []types.TokenPair
	)
	newTokenHexAddr := "0x205CF44075E77A3543abC690437F3b2819bc450a"         //nolint:gosec
	nonExistendTokenHexAddr := "0x8FA78CEB7F04118Ec6d06AaC37Ca854691d8e963" //nolint:gosec
	newTokenDenom := "test"
	tokenPair := types.NewTokenPair(common.HexToAddress(newTokenHexAddr), newTokenDenom, types.OWNER_MODULE)

	testCases := []struct {
		name          string
		paramsFun     func()
		precompile    common.Address
		expectedFound bool
		expectedError bool
		err           string
	}{
		{
			"fail - precompile not on params",
			func() {
				params := types.DefaultParams()
				err := suite.network.App.Erc20Keeper.SetParams(ctx, params)
				suite.Require().NoError(err)
			},
			common.HexToAddress(nonExistendTokenHexAddr),
			false,
			false,
			"",
		},
		{
			"fail - precompile on params, but token pair doesn't exist",
			func() {
				err := suite.network.App.Erc20Keeper.EnableNativePrecompile(ctx, common.HexToAddress(newTokenHexAddr))
				suite.Require().NoError(err)
				err = suite.network.App.Erc20Keeper.EnableNativePrecompile(ctx, common.HexToAddress(nonExistendTokenHexAddr))
				suite.Require().NoError(err)
			},
			common.HexToAddress(nonExistendTokenHexAddr),
			false,
			true,
			"precompiled contract not initialized",
		},
		{
			"success - precompile on params, and token pair exist",
			func() {
				err := suite.network.App.Erc20Keeper.EnableNativePrecompile(ctx, common.HexToAddress(tokenPair.Erc20Address))
				suite.Require().NoError(err)
			},
			common.HexToAddress(tokenPair.Erc20Address),
			true,
			false,
			"",
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()

			err := suite.network.App.Erc20Keeper.SetToken(ctx, tokenPair)
			suite.Require().NoError(err)
			tokenPairs = suite.network.App.Erc20Keeper.GetTokenPairs(ctx)
			suite.Require().True(len(tokenPairs) > 1,
				"expected more than 1 token pair to be set; got %d",
				len(tokenPairs),
			)

			tc.paramsFun()

			_, found, err := suite.network.App.Erc20Keeper.GetERC20PrecompileInstance(ctx, tc.precompile)
			suite.Require().Equal(found, tc.expectedFound)
			if tc.expectedError {
				suite.Require().ErrorContains(err, tc.err)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestGetNativePrecompiles() {
	var ctx sdk.Context
	testAddr := utiltx.GenerateAddress()
	defaultWEVMOSAddr := common.HexToAddress(testconstants.WEVMOSContractMainnet)

	testCases := []struct {
		name     string
		malleate func()
		expRes   []string
	}{
		{
			"default native precompiles registered",
			func() {},
			[]string{defaultWEVMOSAddr.Hex()},
		},
		{
			"no native precompiles registered",
			func() {
				suite.network.App.Erc20Keeper.DeleteNativePrecompile(ctx, defaultWEVMOSAddr)
			},
			nil,
		},
		{
			"multiple native precompiles available",
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{defaultWEVMOSAddr.Hex(), testAddr.Hex()},
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			res := suite.network.App.Erc20Keeper.GetNativePrecompiles(ctx)
			suite.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (suite *KeeperTestSuite) TestSetNativePrecompile() {
	var ctx sdk.Context
	testAddr := utiltx.GenerateAddress()
	defaultWEVMOSAddr := common.HexToAddress(testconstants.WEVMOSContractMainnet)

	testCases := []struct {
		name     string
		addrs    []common.Address
		malleate func()
		expRes   []string
	}{
		{
			"set new native precompile",
			[]common.Address{testAddr},
			func() {},
			[]string{defaultWEVMOSAddr.Hex(), testAddr.Hex()},
		},
		{
			"set duplicate native precompile",
			[]common.Address{testAddr},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{defaultWEVMOSAddr.Hex(), testAddr.Hex()},
		},
		{
			"set non-eip55 native precompile variations",
			[]common.Address{
				common.HexToAddress(strings.ToLower(testAddr.Hex())),
				common.HexToAddress(strings.ToUpper(testAddr.Hex())),
			},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{defaultWEVMOSAddr.Hex(), testAddr.Hex()},
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			for _, addr := range tc.addrs {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, addr)
			}
			res := suite.network.App.Erc20Keeper.GetNativePrecompiles(ctx)
			suite.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (suite *KeeperTestSuite) TestDeleteNativePrecompile() {
	var ctx sdk.Context
	testAddr := utiltx.GenerateAddress()
	defaultWEVMOSAddr := common.HexToAddress(testconstants.WEVMOSContractMainnet)
	unavailableAddr := common.HexToAddress("unavailable")

	testCases := []struct {
		name     string
		addrs    []common.Address
		malleate func()
		expRes   []string
	}{
		{
			"delete all native precompiles",
			[]common.Address{defaultWEVMOSAddr, testAddr},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			nil,
		},
		{
			"delete unavailable native precompile",
			[]common.Address{unavailableAddr},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{defaultWEVMOSAddr.Hex(), testAddr.Hex()},
		},
		{
			"delete default native precompile",
			[]common.Address{defaultWEVMOSAddr},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
		{
			"delete new native precompile",
			[]common.Address{testAddr},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{defaultWEVMOSAddr.Hex()},
		},
		{
			"delete with non-eip55 native precompile lower variation",
			[]common.Address{
				common.HexToAddress(strings.ToLower(defaultWEVMOSAddr.Hex())),
			},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
		{
			"delete with non-eip55 native precompile upper variation",
			[]common.Address{
				common.HexToAddress(strings.ToUpper(defaultWEVMOSAddr.Hex())),
			},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
		{
			"delete multiple of same native precompile",
			[]common.Address{
				defaultWEVMOSAddr,
				defaultWEVMOSAddr,
				defaultWEVMOSAddr,
			},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			for _, addr := range tc.addrs {
				suite.network.App.Erc20Keeper.DeleteNativePrecompile(ctx, addr)
			}
			res := suite.network.App.Erc20Keeper.GetNativePrecompiles(ctx)
			suite.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (suite *KeeperTestSuite) TestIsNativePrecompileAvailable() {
	var ctx sdk.Context
	testAddr := utiltx.GenerateAddress()
	defaultWEVMOSAddr := common.HexToAddress(testconstants.WEVMOSContractMainnet)
	unavailableAddr := common.HexToAddress("unavailable")

	testCases := []struct {
		name     string
		addrs    []common.Address
		malleate func()
		expRes   []bool
	}{
		{
			"all native precompiles are available",
			[]common.Address{defaultWEVMOSAddr, testAddr},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]bool{true, true},
		},
		{
			"only default native precompile is available",
			[]common.Address{defaultWEVMOSAddr, testAddr},
			func() {},
			[]bool{true, false},
		},
		{
			"unavailable native precompile is unavailable",
			[]common.Address{unavailableAddr},
			func() {},
			[]bool{false},
		},
		{
			"non-eip55 native precompiles are available",
			[]common.Address{
				testAddr,
				common.HexToAddress(strings.ToLower(testAddr.Hex())),
				common.HexToAddress(strings.ToUpper(testAddr.Hex())),
			},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]bool{true, true, true},
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			tc.malleate()

			res := make([]bool, 0)
			for _, x := range tc.addrs {
				res = append(res, suite.network.App.Erc20Keeper.IsNativePrecompileAvailable(ctx, x))
			}

			suite.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (suite *KeeperTestSuite) TestGetDynamicPrecompiles() {
	var ctx sdk.Context
	testAddr := utiltx.GenerateAddress()

	testCases := []struct {
		name     string
		malleate func()
		expRes   []string
	}{
		{
			"no dynamic precompiles registered",
			func() {},
			nil,
		},
		{
			"dynamic precompile available",
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			res := suite.network.App.Erc20Keeper.GetDynamicPrecompiles(ctx)
			suite.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (suite *KeeperTestSuite) TestSetDynamicPrecompile() {
	var ctx sdk.Context
	testAddr := utiltx.GenerateAddress()

	testCases := []struct {
		name     string
		addrs    []common.Address
		malleate func()
		expRes   []string
	}{
		{
			"set new dynamic precompile",
			[]common.Address{testAddr},
			func() {},
			[]string{testAddr.Hex()},
		},
		{
			"set duplicate dynamic precompile",
			[]common.Address{testAddr},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
		{
			"set non-eip55 dynamic precompile variations",
			[]common.Address{
				common.HexToAddress(strings.ToLower(testAddr.Hex())),
				common.HexToAddress(strings.ToUpper(testAddr.Hex())),
			},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			for _, addr := range tc.addrs {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, addr)
			}
			res := suite.network.App.Erc20Keeper.GetDynamicPrecompiles(ctx)
			suite.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (suite *KeeperTestSuite) TestDeleteDynamicPrecompile() {
	var ctx sdk.Context
	testAddr := utiltx.GenerateAddress()
	unavailableAddr := common.HexToAddress("unavailable")

	testCases := []struct {
		name     string
		addrs    []common.Address
		malleate func()
		expRes   []string
	}{
		{
			"delete new dynamic precompiles",
			[]common.Address{testAddr},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			nil,
		},
		{
			"delete unavailable dynamic precompile",
			[]common.Address{unavailableAddr},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
		{
			"delete with non-eip55 dynamic precompile lower variation",
			[]common.Address{
				common.HexToAddress(strings.ToLower(testAddr.Hex())),
			},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			nil,
		},
		{
			"delete with non-eip55 dynamic precompile upper variation",
			[]common.Address{
				common.HexToAddress(strings.ToUpper(testAddr.Hex())),
			},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			nil,
		},
		{
			"delete multiple of same dynamic precompile",
			[]common.Address{
				testAddr,
				testAddr,
				testAddr,
			},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			nil,
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			for _, addr := range tc.addrs {
				suite.network.App.Erc20Keeper.DeleteDynamicPrecompile(ctx, addr)
			}
			res := suite.network.App.Erc20Keeper.GetDynamicPrecompiles(ctx)
			suite.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (suite *KeeperTestSuite) TestIsDynamicPrecompileAvailable() {
	var ctx sdk.Context
	testAddr := utiltx.GenerateAddress()
	unavailableAddr := common.HexToAddress("unavailable")

	testCases := []struct {
		name     string
		addrs    []common.Address
		malleate func()
		expRes   []bool
	}{
		{
			"new dynamic precompile is available",
			[]common.Address{testAddr},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			[]bool{true},
		},
		{
			"unavailable dynamic precompile is unavailable",
			[]common.Address{unavailableAddr},
			func() {},
			[]bool{false},
		},
		{
			"non-eip55 dynamic precompiles are available",
			[]common.Address{
				testAddr,
				common.HexToAddress(strings.ToLower(testAddr.Hex())),
				common.HexToAddress(strings.ToUpper(testAddr.Hex())),
			},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			[]bool{true, true, true},
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			tc.malleate()

			res := make([]bool, 0)
			for _, x := range tc.addrs {
				res = append(res, suite.network.App.Erc20Keeper.IsDynamicPrecompileAvailable(ctx, x))
			}

			suite.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}
