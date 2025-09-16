package types_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"

	testconfig "github.com/cosmos/evm/testutil/config"
	"github.com/cosmos/evm/x/vm/types"
)

func TestEvmConfigApply(t *testing.T) {
	evmConfigurator := types.NewEvmConfig().
		WithEVMCoinInfo(testconfig.ExampleChainCoinInfo[testconfig.ExampleChainID])
	err := evmConfigurator.Apply()
	require.NoError(t, err)

	err = evmConfigurator.Apply()
	require.Error(t, err)
	require.Contains(t, err.Error(), "sealed", "expected different error")
}

func TestExtendedEips(t *testing.T) {
	testCases := []struct {
		name        string
		malleate    func() *types.EvmConfig
		expPass     bool
		errContains string
	}{
		{
			"fail - eip already present in activators return an error",
			func() *types.EvmConfig {
				extendedEIPs := map[int]func(*vm.JumpTable){
					3855: func(_ *vm.JumpTable) {},
				}
				ec := types.NewEvmConfig().
					WithEVMCoinInfo(testconfig.ExampleChainCoinInfo[testconfig.ExampleChainID]).
					WithExtendedEips(extendedEIPs)
				return ec
			},
			false,
			"duplicate activation",
		},
		{
			"success - new default extra eips without duplication added",
			func() *types.EvmConfig {
				extendedEIPs := map[int]func(*vm.JumpTable){
					0o000: func(_ *vm.JumpTable) {},
				}
				ec := types.NewEvmConfig().
					WithEVMCoinInfo(testconfig.ExampleChainCoinInfo[testconfig.ExampleChainID]).
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
		err := ec.Apply()

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
		malleate    func() *types.EvmConfig
		postCheck   func()
		expPass     bool
		errContains string
	}{
		{
			"fail - duplicate default EIP entiries",
			func() *types.EvmConfig {
				extendedDefaultExtraEIPs := []int64{1000}
				types.DefaultExtraEIPs = append(types.DefaultExtraEIPs, 1000)
				ec := types.NewEvmConfig().
					WithEVMCoinInfo(testconfig.ExampleChainCoinInfo[testconfig.ExampleChainID]).
					WithExtendedDefaultExtraEIPs(extendedDefaultExtraEIPs...)
				return ec
			},
			func() {
				require.ElementsMatch(t, append(defaultExtraEIPsSnapshot, 1000), types.DefaultExtraEIPs)
				types.DefaultExtraEIPs = defaultExtraEIPsSnapshot
			},
			false,
			"EIP 1000 is already present",
		},
		{
			"success - empty default extra eip",
			func() *types.EvmConfig {
				var extendedDefaultExtraEIPs []int64
				ec := types.NewEvmConfig().
					WithEVMCoinInfo(testconfig.ExampleChainCoinInfo[testconfig.ExampleChainID]).
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
			func() *types.EvmConfig {
				extendedDefaultExtraEIPs := []int64{1001}
				ec := types.NewEvmConfig().
					WithEVMCoinInfo(testconfig.ExampleChainCoinInfo[testconfig.ExampleChainID]).
					WithExtendedDefaultExtraEIPs(extendedDefaultExtraEIPs...)
				return ec
			},
			func() {
				require.ElementsMatch(t, append(defaultExtraEIPsSnapshot, 1001), types.DefaultExtraEIPs)
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
			err := ec.Apply()

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
