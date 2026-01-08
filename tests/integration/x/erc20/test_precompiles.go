package erc20

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ethereum/go-ethereum/common"

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
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()

			err := s.network.App.GetErc20Keeper().SetToken(ctx, tokenPair)
			s.Require().NoError(err)
			tokenPairs = s.network.App.GetErc20Keeper().GetTokenPairs(ctx)
			s.Require().True(len(tokenPairs) == 1,
				"expected 1 token pair to be set; got %d",
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
