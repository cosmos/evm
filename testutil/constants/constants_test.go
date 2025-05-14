package constants_test

import (
	config2 "github.com/cosmos/evm/cmd/evmd/config"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/testutil/constants"
)

func TestRequireSameTestDenom(t *testing.T) {
	require.Equal(t,
		constants.ExampleAttoDenom,
		config2.ExampleChainDenom,
		"test denoms should be the same across the repo",
	)
}

func TestRequireSameTestBech32Prefix(t *testing.T) {
	require.Equal(t,
		constants.ExampleBech32Prefix,
		config2.Bech32Prefix,
		"bech32 prefixes should be the same across the repo",
	)
}

func TestRequireSameWEVMOSMainnet(t *testing.T) {
	require.Equal(t,
		constants.WEVMOSContractMainnet,
		config2.WEVMOSContractMainnet,
		"wevmos contract addresses should be the same across the repo",
	)
}
