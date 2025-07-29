package keeper_test

import (
	exampleapp "github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/x/vm/types"
)

func (suite *KeeperTestSuite) TestParams() {
	defaultChainEVMParams := exampleapp.NewEVMGenesisState().Params
	defaultChainEVMParams.ActiveStaticPrecompiles = types.AvailableStaticPrecompiles

	testCases := []struct {
		name      string
		paramsFun func() interface{}
		getFun    func() interface{}
		expected  bool
	}{
		{
			"success - Checks if the default params are set correctly",
			func() interface{} {
				return defaultChainEVMParams
			},
			func() interface{} {
				return suite.network.App.EVMKeeper.GetParams(suite.network.GetContext())
			},
			true,
		},
		{
			"success - Check Access Control Create param is set to restricted and can be retrieved correctly",
			func() interface{} {
				params := defaultChainEVMParams
				params.AccessControl = types.AccessControl{
					Create: types.AccessControlType{
						AccessType: types.AccessTypeRestricted,
					},
				}
				err := suite.network.App.EVMKeeper.SetParams(suite.network.GetContext(), params)
				suite.Require().NoError(err)
				return types.AccessTypeRestricted
			},
			func() interface{} {
				evmParams := suite.network.App.EVMKeeper.GetParams(suite.network.GetContext())
				return evmParams.GetAccessControl().Create.AccessType
			},
			true,
		},
		{
			"success - Check Access control param is set to restricted and can be retrieved correctly",
			func() interface{} {
				params := defaultChainEVMParams
				params.AccessControl = types.AccessControl{
					Call: types.AccessControlType{
						AccessType: types.AccessTypeRestricted,
					},
				}
				err := suite.network.App.EVMKeeper.SetParams(suite.network.GetContext(), params)
				suite.Require().NoError(err)
				return types.AccessTypeRestricted
			},
			func() interface{} {
				evmParams := suite.network.App.EVMKeeper.GetParams(suite.network.GetContext())
				return evmParams.GetAccessControl().Call.AccessType
			},
			true,
		},
		{
			"success - Check AllowUnprotectedTxs param is set to false and can be retrieved correctly",
			func() interface{} {
				params := defaultChainEVMParams
				params.AllowUnprotectedTxs = false
				err := suite.network.App.EVMKeeper.SetParams(suite.network.GetContext(), params)
				suite.Require().NoError(err)
				return params.AllowUnprotectedTxs
			},
			func() interface{} {
				evmParams := suite.network.App.EVMKeeper.GetParams(suite.network.GetContext())
				return evmParams.GetAllowUnprotectedTxs()
			},
			true,
		},
		{
			name: "success - Active precompiles are sorted when setting params",
			paramsFun: func() interface{} {
				params := defaultChainEVMParams
				params.ActiveStaticPrecompiles = []string{
					"0x0000000000000000000000000000000000000801",
					"0x0000000000000000000000000000000000000800",
				}
				err := suite.network.App.EVMKeeper.SetParams(suite.network.GetContext(), params)
				suite.Require().NoError(err, "expected no error when setting params")

				// NOTE: return sorted slice here because the precompiles should be sorted when setting the params
				return []string{
					"0x0000000000000000000000000000000000000800",
					"0x0000000000000000000000000000000000000801",
				}
			},
			getFun: func() interface{} {
				evmParams := suite.network.App.EVMKeeper.GetParams(suite.network.GetContext())
				return evmParams.GetActiveStaticPrecompiles()
			},
			expected: true,
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			suite.Require().Equal(tc.paramsFun(), tc.getFun(), "expected different params")
		})
	}
}
