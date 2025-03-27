package constants_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	chainconfig "github.com/cosmos/evm/cmd/evmd/config"
	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/testutil/constants"
)

func TestRequireSameTestDenom(t *testing.T) {
	require.Equal(t,
		constants.ExampleAttoDenom,
		evmd.ExampleChainDenom,
		"test denoms should be the same across the repo",
	)
}

func TestRequireSameTestBech32Prefix(t *testing.T) {
	require.Equal(t,
		constants.ExampleBech32Prefix,
		chainconfig.Bech32Prefix,
		"bech32 prefixes should be the same across the repo",
	)
}

func TestRequireSameWEVMOSMainnet(t *testing.T) {
	require.Equal(t,
		constants.WEVMOSContractMainnet,
		evmd.WEVMOSContractMainnet,
		"wevmos contract addresses should be the same across the repo",
	)
}
