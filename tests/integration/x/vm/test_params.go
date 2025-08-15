package vm

import (
	"github.com/cosmos/evm/testutil/config"
	"github.com/cosmos/evm/x/vm/types"
)

func (s *KeeperTestSuite) TestParams() {
	defaultChainEVMParams := config.NewEVMGenesisState().Params
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
				return s.Network.App.GetEVMKeeper().GetParams(s.Network.GetContext())
			},
			true,
		},
		{
			"success - Check Access Control create param is set to restricted and can be retrieved correctly",
			func() interface{} {
				params := defaultChainEVMParams
				params.AccessControl = types.AccessControl{
					Create: types.AccessControlType{
						AccessType: types.AccessTypeRestricted,
					},
				}
				err := s.Network.App.GetEVMKeeper().SetParams(s.Network.GetContext(), params)
				s.Require().NoError(err)
				return types.AccessTypeRestricted
			},
			func() interface{} {
				evmParams := s.Network.App.GetEVMKeeper().GetParams(s.Network.GetContext())
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
				err := s.Network.App.GetEVMKeeper().SetParams(s.Network.GetContext(), params)
				s.Require().NoError(err)
				return types.AccessTypeRestricted
			},
			func() interface{} {
				evmParams := s.Network.App.GetEVMKeeper().GetParams(s.Network.GetContext())
				return evmParams.GetAccessControl().Call.AccessType
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
				err := s.Network.App.GetEVMKeeper().SetParams(s.Network.GetContext(), params)
				s.Require().NoError(err, "expected no error when setting params")

				// NOTE: return sorted slice here because the precompiles should be sorted when setting the params
				return []string{
					"0x0000000000000000000000000000000000000800",
					"0x0000000000000000000000000000000000000801",
				}
			},
			getFun: func() interface{} {
				evmParams := s.Network.App.GetEVMKeeper().GetParams(s.Network.GetContext())
				return evmParams.GetActiveStaticPrecompiles()
			},
			expected: true,
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			s.Require().Equal(tc.paramsFun(), tc.getFun(), "expected different params")
		})
	}
}
