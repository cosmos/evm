package config_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/testutil/config"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func TestGenerateConfigFromTemplate(t *testing.T) {
	testCases := []struct {
		name      string
		chainCfg  config.DynamicChainConfig
		expectErr bool
	}{
		{
			name:     "default 18 decimals config",
			chainCfg: config.DefaultTestChain,
		},
		{
			name:     "6 decimals config",
			chainCfg: config.SixDecimalsTestChain,
		},
		{
			name: "custom chain config",
			chainCfg: config.CreateCustomTestChain(
				"mycustomchain-1",
				12345,
				"umycoin",
				"mycoin",
				8,
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tomlContent, err := config.GenerateConfigFromTemplate(tc.chainCfg)

			if tc.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, tomlContent)

			// Verify the generated TOML contains expected values
			require.Contains(t, tomlContent, `denom = "`+tc.chainCfg.Denom+`"`)
			require.Contains(t, tomlContent, `extended-denom = "`+tc.chainCfg.ExtendedDenom+`"`)
			require.Contains(t, tomlContent, `display-denom = "`+tc.chainCfg.DisplayDenom+`"`)
			require.Contains(t, tomlContent, fmt.Sprintf(`decimals = %d`, tc.chainCfg.Decimals))

			// Test that it can be parsed back
			parsedConfig, err := config.ParseConfigFromTOML(tomlContent)
			require.NoError(t, err)
			require.NotNil(t, parsedConfig)

			// Verify parsed values match input
			require.Equal(t, tc.chainCfg.Denom, parsedConfig.Chain.Denom)
			require.Equal(t, tc.chainCfg.ExtendedDenom, parsedConfig.Chain.ExtendedDenom)
			require.Equal(t, tc.chainCfg.DisplayDenom, parsedConfig.Chain.DisplayDenom)
			require.Equal(t, tc.chainCfg.Decimals, parsedConfig.Chain.Decimals)
			require.Equal(t, tc.chainCfg.EVMChainID, parsedConfig.EVM.EVMChainID)
		})
	}
}

func TestCreateEvmCoinInfoFromDynamicConfig(t *testing.T) {
	chainCfg := config.CreateCustomTestChain(
		"testchain-1",
		777,
		"utoken",
		"token",
		12,
	)

	coinInfo := config.CreateEvmCoinInfoFromDynamicConfig(chainCfg)

	require.Equal(t, "utoken", coinInfo.Denom)
	require.Equal(t, "atoken", coinInfo.ExtendedDenom)
	require.Equal(t, "token", coinInfo.DisplayDenom)
	require.Equal(t, evmtypes.Decimals(12), coinInfo.Decimals)
}

func TestGenerateAndParseConfig(t *testing.T) {
	chainCfg := config.CreateCustomTestChain(
		"integration-test-1",
		999,
		"ustake",
		"stake",
		16,
	)

	tomlContent, parsedConfig, err := config.GenerateAndParseConfig(chainCfg)
	require.NoError(t, err)
	require.NotEmpty(t, tomlContent)
	require.NotNil(t, parsedConfig)

	// Verify the round-trip worked correctly
	require.Equal(t, chainCfg.Denom, parsedConfig.Chain.Denom)
	require.Equal(t, chainCfg.EVMChainID, parsedConfig.EVM.EVMChainID)

	// Verify it's valid TOML
	require.True(t, strings.Contains(tomlContent, "[chain]"))
	require.True(t, strings.Contains(tomlContent, "[evm]"))
	require.True(t, strings.Contains(tomlContent, "[json-rpc]"))
}

func TestCreateCustomTestChain(t *testing.T) {
	chainCfg := config.CreateCustomTestChain(
		"arbitrary-chain-id",
		54321,
		"myhundred",
		"hundred",
		2,
	)

	require.Equal(t, "arbitrary-chain-id", chainCfg.ChainID)
	require.Equal(t, uint64(54321), chainCfg.EVMChainID)
	require.Equal(t, "myhundred", chainCfg.Denom)
	require.Equal(t, "ahundred", chainCfg.ExtendedDenom)
	require.Equal(t, "hundred", chainCfg.DisplayDenom)
	require.Equal(t, uint8(2), chainCfg.Decimals)
}
