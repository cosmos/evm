package erc20

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

func (s *KeeperTestSuite) TestGetERC20PrecompileInstance() {
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
				err := s.network.App.GetErc20Keeper().SetParams(ctx, params)
				s.Require().NoError(err)
			},
			common.HexToAddress(nonExistendTokenHexAddr),
			false,
			false,
			"",
		},
		{
			"fail - precompile on params, but token pair doesn't exist",
			func() {
				err := s.network.App.GetErc20Keeper().EnableNativePrecompile(ctx, common.HexToAddress(newTokenHexAddr))
				s.Require().NoError(err)
				err = s.network.App.GetErc20Keeper().EnableNativePrecompile(ctx, common.HexToAddress(nonExistendTokenHexAddr))
				s.Require().NoError(err)
			},
			common.HexToAddress(nonExistendTokenHexAddr),
			false,
			true,
			"precompiled contract not initialized",
		},
		{
			"success - precompile on params, and token pair exist",
			func() {
				err := s.network.App.GetErc20Keeper().EnableNativePrecompile(ctx, common.HexToAddress(tokenPair.Erc20Address))
				s.Require().NoError(err)
			},
			common.HexToAddress(tokenPair.Erc20Address),
			true,
			false,
			"",
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()

			err := s.network.App.GetErc20Keeper().SetToken(ctx, tokenPair)
			s.Require().NoError(err)
			tokenPairs = s.network.App.GetErc20Keeper().GetTokenPairs(ctx)
			s.Require().True(len(tokenPairs) > 1,
				"expected more than 1 token pair to be set; got %d",
				len(tokenPairs),
			)

			tc.paramsFun()

			_, found, err := s.network.App.GetErc20Keeper().GetERC20PrecompileInstance(ctx, tc.precompile)
			s.Require().Equal(found, tc.expectedFound)
			if tc.expectedError {
				s.Require().ErrorContains(err, tc.err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestGetNativePrecompiles() {
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
				s.network.App.GetErc20Keeper().DeleteNativePrecompile(ctx, defaultWEVMOSAddr)
			},
			nil,
		},
		{
			"multiple native precompiles available",
			func() {
				s.network.App.GetErc20Keeper().SetNativePrecompile(ctx, testAddr)
			},
			[]string{defaultWEVMOSAddr.Hex(), testAddr.Hex()},
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			res := s.network.App.GetErc20Keeper().GetNativePrecompiles(ctx)
			s.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (s *KeeperTestSuite) TestSetNativePrecompile() {
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
				s.network.App.GetErc20Keeper().SetNativePrecompile(ctx, testAddr)
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
				s.network.App.GetErc20Keeper().SetNativePrecompile(ctx, testAddr)
			},
			[]string{defaultWEVMOSAddr.Hex(), testAddr.Hex()},
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			for _, addr := range tc.addrs {
				s.network.App.GetErc20Keeper().SetNativePrecompile(ctx, addr)
			}
			res := s.network.App.GetErc20Keeper().GetNativePrecompiles(ctx)
			s.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (s *KeeperTestSuite) TestDeleteNativePrecompile() {
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
				s.network.App.GetErc20Keeper().SetNativePrecompile(ctx, testAddr)
			},
			nil,
		},
		{
			"delete unavailable native precompile",
			[]common.Address{unavailableAddr},
			func() {
				s.network.App.GetErc20Keeper().SetNativePrecompile(ctx, testAddr)
			},
			[]string{defaultWEVMOSAddr.Hex(), testAddr.Hex()},
		},
		{
			"delete default native precompile",
			[]common.Address{defaultWEVMOSAddr},
			func() {
				s.network.App.GetErc20Keeper().SetNativePrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
		{
			"delete new native precompile",
			[]common.Address{testAddr},
			func() {
				s.network.App.GetErc20Keeper().SetNativePrecompile(ctx, testAddr)
			},
			[]string{defaultWEVMOSAddr.Hex()},
		},
		{
			"delete with non-eip55 native precompile lower variation",
			[]common.Address{
				common.HexToAddress(strings.ToLower(defaultWEVMOSAddr.Hex())),
			},
			func() {
				s.network.App.GetErc20Keeper().SetNativePrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
		{
			"delete with non-eip55 native precompile upper variation",
			[]common.Address{
				common.HexToAddress(strings.ToUpper(defaultWEVMOSAddr.Hex())),
			},
			func() {
				s.network.App.GetErc20Keeper().SetNativePrecompile(ctx, testAddr)
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
				s.network.App.GetErc20Keeper().SetNativePrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			for _, addr := range tc.addrs {
				s.network.App.GetErc20Keeper().DeleteNativePrecompile(ctx, addr)
			}
			res := s.network.App.GetErc20Keeper().GetNativePrecompiles(ctx)
			s.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (s *KeeperTestSuite) TestIsNativePrecompileAvailable() {
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
				s.network.App.GetErc20Keeper().SetNativePrecompile(ctx, testAddr)
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
				s.network.App.GetErc20Keeper().SetNativePrecompile(ctx, testAddr)
			},
			[]bool{true, true, true},
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			tc.malleate()

			res := make([]bool, 0)
			for _, x := range tc.addrs {
				res = append(res, s.network.App.GetErc20Keeper().IsNativePrecompileAvailable(ctx, x))
			}

			s.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (s *KeeperTestSuite) TestGetDynamicPrecompiles() {
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
				s.network.App.GetErc20Keeper().SetDynamicPrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			res := s.network.App.GetErc20Keeper().GetDynamicPrecompiles(ctx)
			s.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (s *KeeperTestSuite) TestSetDynamicPrecompile() {
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
				s.network.App.GetErc20Keeper().SetDynamicPrecompile(ctx, testAddr)
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
				s.network.App.GetErc20Keeper().SetDynamicPrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			for _, addr := range tc.addrs {
				s.network.App.GetErc20Keeper().SetDynamicPrecompile(ctx, addr)
			}
			res := s.network.App.GetErc20Keeper().GetDynamicPrecompiles(ctx)
			s.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (s *KeeperTestSuite) TestDeleteDynamicPrecompile() {
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
				s.network.App.GetErc20Keeper().SetDynamicPrecompile(ctx, testAddr)
			},
			nil,
		},
		{
			"delete unavailable dynamic precompile",
			[]common.Address{unavailableAddr},
			func() {
				s.network.App.GetErc20Keeper().SetDynamicPrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
		{
			"delete with non-eip55 dynamic precompile lower variation",
			[]common.Address{
				common.HexToAddress(strings.ToLower(testAddr.Hex())),
			},
			func() {
				s.network.App.GetErc20Keeper().SetDynamicPrecompile(ctx, testAddr)
			},
			nil,
		},
		{
			"delete with non-eip55 dynamic precompile upper variation",
			[]common.Address{
				common.HexToAddress(strings.ToUpper(testAddr.Hex())),
			},
			func() {
				s.network.App.GetErc20Keeper().SetDynamicPrecompile(ctx, testAddr)
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
				s.network.App.GetErc20Keeper().SetDynamicPrecompile(ctx, testAddr)
			},
			nil,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			for _, addr := range tc.addrs {
				s.network.App.GetErc20Keeper().DeleteDynamicPrecompile(ctx, addr)
			}
			res := s.network.App.GetErc20Keeper().GetDynamicPrecompiles(ctx)
			s.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (s *KeeperTestSuite) TestIsDynamicPrecompileAvailable() {
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
				s.network.App.GetErc20Keeper().SetDynamicPrecompile(ctx, testAddr)
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
				s.network.App.GetErc20Keeper().SetDynamicPrecompile(ctx, testAddr)
			},
			[]bool{true, true, true},
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			tc.malleate()

			res := make([]bool, 0)
			for _, x := range tc.addrs {
				res = append(res, s.network.App.GetErc20Keeper().IsDynamicPrecompileAvailable(ctx, x))
			}

			s.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}
