package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/vm/core/vm"
	"github.com/cosmos/evm/x/vm/types"
)

func TestEVMConfigurator(t *testing.T) {
	evmConfigurator := types.NewEVMConfigurator().
		WithEVMCoinInfo(testconstants.ExampleAttoDenom, uint8(types.EighteenDecimals))
	err := evmConfigurator.Configure()
	require.NoError(t, err)

	err = evmConfigurator.Configure()
	require.Error(t, err)
	require.Contains(t, err.Error(), "sealed", "expected different error")
}

func TestExtendedEips(t *testing.T) {
	testCases := []struct {
		name        string
		malleate    func() *types.EVMConfigurator
		expPass     bool
		errContains string
	}{
		{
			"fail - eip already present in activators return an error",
			func() *types.EVMConfigurator {
				extendedEIPs := map[string]func(*vm.JumpTable){
					"ethereum_3855": func(_ *vm.JumpTable) {},
				}
				ec := types.NewEVMConfigurator().
					WithEVMCoinInfo(testconstants.ExampleAttoDenom, uint8(types.EighteenDecimals)).
					WithExtendedEips(extendedEIPs)
				return ec
			},
			false,
			"duplicate activation",
		},
		{
			"success - new default extra eips without duplication added",
			func() *types.EVMConfigurator {
				extendedEIPs := map[string]func(*vm.JumpTable){
					"evmos_0": func(_ *vm.JumpTable) {},
				}
				ec := types.NewEVMConfigurator().
					WithEVMCoinInfo(testconstants.ExampleAttoDenom, uint8(types.EighteenDecimals)).
					WithExtendedEips(extendedEIPs)
				return ec
			},
			true,
			"",
		},
	}

	for _, tc := range testCases {
		ec := tc.malleate()
		ec.ResetTestConfig()
		err := ec.Configure()

		if tc.expPass {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.errContains, "expected different error")
		}
	}
}

func TestExtendedDefaultExtraEips(t *testing.T) {
	defaultExtraEIPsSnapshot := types.DefaultExtraEIPs
	testCases := []struct {
		name        string
		malleate    func() *types.EVMConfigurator
		postCheck   func()
		expPass     bool
		errContains string
	}{
		{
			"fail - invalid eip name",
			func() *types.EVMConfigurator {
				extendedDefaultExtraEIPs := []string{"cosmos_1_000"}
				ec := types.NewEVMConfigurator().
					WithEVMCoinInfo(testconstants.ExampleAttoDenom, uint8(types.EighteenDecimals)).
					WithExtendedDefaultExtraEIPs(extendedDefaultExtraEIPs...)
				return ec
			},
			func() {
				require.ElementsMatch(t, defaultExtraEIPsSnapshot, types.DefaultExtraEIPs)
				types.DefaultExtraEIPs = defaultExtraEIPsSnapshot
			},
			false,
			"eip name does not conform to structure",
		},
		{
			"fail - duplicate default EIP entiries",
			func() *types.EVMConfigurator {
				extendedDefaultExtraEIPs := []string{"cosmos_1000"}
				types.DefaultExtraEIPs = append(types.DefaultExtraEIPs, "cosmos_1000")
				ec := types.NewEVMConfigurator().
					WithEVMCoinInfo(testconstants.ExampleAttoDenom, uint8(types.EighteenDecimals)).
					WithExtendedDefaultExtraEIPs(extendedDefaultExtraEIPs...)
				return ec
			},
			func() {
				require.ElementsMatch(t, append(defaultExtraEIPsSnapshot, "cosmos_1000"), types.DefaultExtraEIPs)
				types.DefaultExtraEIPs = defaultExtraEIPsSnapshot
			},
			false,
			"EIP cosmos_1000 is already present",
		},
		{
			"success - empty default extra eip",
			func() *types.EVMConfigurator {
				var extendedDefaultExtraEIPs []string
				ec := types.NewEVMConfigurator().
					WithEVMCoinInfo(testconstants.ExampleAttoDenom, uint8(types.EighteenDecimals)).
					WithExtendedDefaultExtraEIPs(extendedDefaultExtraEIPs...)
				return ec
			},
			func() {
				require.ElementsMatch(t, defaultExtraEIPsSnapshot, types.DefaultExtraEIPs)
			},
			true,
			"",
		},
		{
			"success - extra default eip added",
			func() *types.EVMConfigurator {
				extendedDefaultExtraEIPs := []string{"os_1001"}
				ec := types.NewEVMConfigurator().
					WithEVMCoinInfo(testconstants.ExampleAttoDenom, uint8(types.EighteenDecimals)).
					WithExtendedDefaultExtraEIPs(extendedDefaultExtraEIPs...)
				return ec
			},
			func() {
				require.ElementsMatch(t, append(defaultExtraEIPsSnapshot, "os_1001"), types.DefaultExtraEIPs)
				types.DefaultExtraEIPs = defaultExtraEIPsSnapshot
			},
			true,
			"",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ec := tc.malleate()
			ec.ResetTestConfig()
			err := ec.Configure()

			if tc.expPass {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errContains, "expected different error")
			}

			tc.postCheck()
		})
	}
}
