package erc20

import (
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/erc20/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperTestSuite) TestParams() {
	var ctx sdk.Context

	testCases := []struct {
		name      string
		paramsFun func() interface{}
		getFun    func() interface{}
		expected  bool
	}{
		{
			"success - Checks if the default params are set correctly",
			func() interface{} {
				erc20Params := types.DefaultParams()
				// NOTE: we need to add the example token pair address which is not in the default params but in the genesis state
				// of the test s app and therefore is returned by the query client.
				erc20Params.NativePrecompiles = append(erc20Params.NativePrecompiles, testconstants.WEVMOSContractMainnet)

				return erc20Params
			},
			func() interface{} {
				return s.network.App.GetErc20Keeper().GetParams(ctx)
			},
			true,
		},
		{
			"success - Checks if dynamic precompiles are set correctly",
			func() interface{} {
				params := types.DefaultParams()
				params.DynamicPrecompiles = []string{"0xB5124FA2b2cF92B2D469b249433BA1c96BDF536D", "0xC4CcDf91b810a61cCB48b35ccCc066C63bf94B4F"}
				err := s.network.App.GetErc20Keeper().SetParams(ctx, params)
				s.Require().NoError(err)
				return params.DynamicPrecompiles
			},
			func() interface{} {
				return s.network.App.GetErc20Keeper().GetParams(ctx).DynamicPrecompiles
			},
			true,
		},
		{
			"success - Checks if native precompiles are set correctly",
			func() interface{} {
				params := types.DefaultParams()
				params.NativePrecompiles = []string{"0x205CF44075E77A3543abC690437F3b2819bc450a", "0x8FA78CEB7F04118Ec6d06AaC37Ca854691d8e963"}
				err := s.network.App.GetErc20Keeper().SetParams(ctx, params)
				s.Require().NoError(err)
				return params.NativePrecompiles
			},
			func() interface{} {
				return s.network.App.GetErc20Keeper().GetParams(ctx).NativePrecompiles
			},
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()

			s.Require().Equal(tc.paramsFun(), tc.getFun())
		})
	}
}
