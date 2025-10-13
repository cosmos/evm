package types_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/erc20/types"

	"cosmossdk.io/math"
)

type GenesisTestSuite struct {
	suite.Suite
}

func (suite *GenesisTestSuite) SetupTest() {
}

func TestGenesisTestSuite(t *testing.T) {
	suite.Run(t, new(GenesisTestSuite))
}

func (suite *GenesisTestSuite) TestValidateGenesis() {
	newGen := types.NewGenesisState(types.DefaultParams(), testconstants.ExampleTokenPairs, testconstants.ExampleAllowances)

	testCases := []struct {
		name     string
		genState *types.GenesisState
		expPass  bool
	}{
		{
			name:     "valid genesis constructor",
			genState: &newGen,
			expPass:  true,
		},
		{
			name:     "default",
			genState: types.DefaultGenesisState(),
			expPass:  true,
		},
		{
			name: "valid genesis",
			genState: &types.GenesisState{
				Params:     types.DefaultParams(),
				TokenPairs: testconstants.ExampleTokenPairs,
				Allowances: testconstants.ExampleAllowances,
			},
			expPass: true,
		},
		{
			name: "valid genesis - with tokens pairs",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				TokenPairs: []types.TokenPair{
					{
						Erc20Address: "0xdac17f958d2ee523a2206206994597c13d831ec7",
						Denom:        "usdt",
						Enabled:      true,
					},
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Denom:        testconstants.ExampleAttoDenom,
						Enabled:      true,
					},
				},
				Allowances: testconstants.ExampleAllowances,
			},
			expPass: true,
		},
		{
			name: "invalid genesis - duplicated token pair",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				TokenPairs: []types.TokenPair{
					{
						Erc20Address: "0xdac17f958d2ee523a2206206994597c13d831ec7",
						Denom:        "usdt",
						Enabled:      true,
					},
					{
						Erc20Address: "0xdac17f958d2ee523a2206206994597c13d831ec7",
						Denom:        "usdt",
						Enabled:      true,
					},
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Denom:        testconstants.ExampleAttoDenom,
						Enabled:      true,
					},
				},
				Allowances: testconstants.ExampleAllowances,
			},
			expPass: false,
		},
		{
			name: "invalid genesis - duplicated token pair",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				TokenPairs: []types.TokenPair{
					{
						Erc20Address: "0xdac17f958d2ee523a2206206994597c13d831ec7",
						Denom:        "usdt",
						Enabled:      true,
					},
					{
						Erc20Address: "0xdac17f958d2ee523a2206206994597c13d831ec7",
						Denom:        "usdt2",
						Enabled:      true,
					},
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Denom:        testconstants.ExampleAttoDenom,
						Enabled:      true,
					},
				},
				Allowances: testconstants.ExampleAllowances,
			},
			expPass: false,
		},
		{
			name: "invalid genesis - duplicated token pair",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				TokenPairs: []types.TokenPair{
					{
						Erc20Address: "0xdac17f958d2ee523a2206206994597c13d831ec7",
						Denom:        "usdt",
						Enabled:      true,
					},
					{
						Erc20Address: "0xB8c77482e45F1F44dE1745F52C74426C631bDD52",
						Denom:        "usdt",
						Enabled:      true,
					},
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Denom:        testconstants.ExampleAttoDenom,
						Enabled:      true,
					},
				},
				Allowances: testconstants.ExampleAllowances,
			},
			expPass: false,
		},
		{
			name: "invalid genesis - invalid token pair",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				TokenPairs: []types.TokenPair{
					{
						Erc20Address: "0xinvalidaddress",
						Denom:        "bad",
						Enabled:      true,
					},
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Denom:        testconstants.ExampleAttoDenom,
						Enabled:      true,
					},
				},
				Allowances: testconstants.ExampleAllowances,
			},
			expPass: false,
		},
		{
			name: "invalid genesis - missing wevmos token pair",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				TokenPairs: []types.TokenPair{
					{
						Erc20Address: "0xinvalidaddress",
						Denom:        "bad",
						Enabled:      true,
					},
				},
				Allowances: testconstants.ExampleAllowances,
			},
			expPass: false,
		},
		{
			name: "invalid genesis - duplicated allowances",
			genState: &types.GenesisState{
				Params:     types.DefaultParams(),
				TokenPairs: testconstants.ExampleTokenPairs,
				Allowances: []types.Allowance{
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Owner:        testconstants.ExampleEvmAddressAlice,
						Spender:      testconstants.ExampleEvmAddressBob,
						Value:        math.NewInt(100),
					},
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Owner:        testconstants.ExampleEvmAddressAlice,
						Spender:      testconstants.ExampleEvmAddressBob,
						Value:        math.NewInt(100),
					},
				},
			},
			expPass: false,
		},
		{
			name: "invalid genesis - invalid allowance erc20 address",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				TokenPairs: []types.TokenPair{
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Denom:        testconstants.ExampleAttoDenom,
						Enabled:      true,
					},
				},
				Allowances: []types.Allowance{
					{
						Erc20Address: "bad",
						Owner:        testconstants.ExampleEvmAddressAlice,
						Spender:      testconstants.ExampleEvmAddressBob,
						Value:        math.NewInt(-1),
					},
				},
			},
			expPass: false,
		},
		{
			name: "invalid genesis - invalid allowance owner",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				TokenPairs: []types.TokenPair{
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Denom:        testconstants.ExampleAttoDenom,
						Enabled:      true,
					},
				},
				Allowances: []types.Allowance{
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Owner:        "bad",
						Spender:      testconstants.ExampleEvmAddressBob,
						Value:        math.NewInt(-1),
					},
				},
			},
			expPass: false,
		},
		{
			name: "invalid genesis - invalid allowance spender",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				TokenPairs: []types.TokenPair{
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Denom:        testconstants.ExampleAttoDenom,
						Enabled:      true,
					},
				},
				Allowances: []types.Allowance{
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Owner:        testconstants.ExampleEvmAddressAlice,
						Spender:      "bad",
						Value:        math.NewInt(-1),
					},
				},
			},
			expPass: false,
		},
		{
			name: "invalid genesis - invalid allowance value",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				TokenPairs: []types.TokenPair{
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Denom:        testconstants.ExampleAttoDenom,
						Enabled:      true,
					},
				},
				Allowances: []types.Allowance{
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Owner:        testconstants.ExampleEvmAddressAlice,
						Spender:      testconstants.ExampleEvmAddressBob,
						Value:        math.NewInt(0),
					},
				},
			},
			expPass: false,
		},
		{
			name: "invalid genesis - invalid allowance value",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				TokenPairs: []types.TokenPair{
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Denom:        testconstants.ExampleAttoDenom,
						Enabled:      true,
					},
				},
				Allowances: []types.Allowance{
					{
						Erc20Address: testconstants.WEVMOSContractMainnet,
						Owner:        testconstants.ExampleEvmAddressAlice,
						Spender:      testconstants.ExampleEvmAddressBob,
						Value:        math.NewInt(-1),
					},
				},
			},
			expPass: false,
		},
		{
			// Voting period cant be zero
			name:     "empty genesis",
			genState: &types.GenesisState{},
			expPass:  true,
		},
	}

	for _, tc := range testCases {
		err := tc.genState.Validate()
		if tc.expPass {
			suite.Require().NoError(err, tc.name)
		} else {
			suite.Require().Error(err, tc.name)
		}
	}
}
