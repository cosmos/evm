package server

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client/flags"
	sdkserver "github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type mockAppOptions struct {
	values map[string]interface{}
}

func newMockAppOptions() *mockAppOptions {
	return &mockAppOptions{
		values: make(map[string]interface{}),
	}
}

func (m *mockAppOptions) Get(key string) interface{} {
	return m.values[key]
}

func (m *mockAppOptions) Set(key string, value interface{}) {
	m.values[key] = value
}

func TestGetBlockGasLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setupFn  func() servertypes.AppOptions
		expected uint64
	}{
		{
			name: "empty home directory returns max uint64",
			setupFn: func() servertypes.AppOptions {
				opts := newMockAppOptions()
				return opts
			},
			expected: math.MaxUint64,
		},
		{
			name: "genesis file not found returns 0",
			setupFn: func() servertypes.AppOptions {
				opts := newMockAppOptions()
				opts.Set(flags.FlagHome, "/non/existent/directory")
				return opts
			},
			expected: 0,
		},
		{
			name: "valid genesis with max_gas = -1 returns max uint64",
			setupFn: func() servertypes.AppOptions {
				homeDir := createGenesisWithMaxGas(t, -1)
				opts := newMockAppOptions()
				opts.Set(flags.FlagHome, homeDir)
				return opts
			},
			expected: math.MaxUint64,
		},
		{
			name: "valid genesis with max_gas < -1 returns 0",
			setupFn: func() servertypes.AppOptions {
				homeDir := createGenesisWithMaxGas(t, -5)
				opts := newMockAppOptions()
				opts.Set(flags.FlagHome, homeDir)
				return opts
			},
			expected: 0,
		},
		{
			name: "valid genesis with max_gas = 0 returns 0",
			setupFn: func() servertypes.AppOptions {
				homeDir := createGenesisWithMaxGas(t, 0)
				opts := newMockAppOptions()
				opts.Set(flags.FlagHome, homeDir)
				return opts
			},
			expected: 0,
		},
		{
			name: "valid genesis with max_gas = 1000000 returns 1000000",
			setupFn: func() servertypes.AppOptions {
				homeDir := createGenesisWithMaxGas(t, 1000000)
				opts := newMockAppOptions()
				opts.Set(flags.FlagHome, homeDir)
				return opts
			},
			expected: 1000000,
		},
		{
			name: "genesis without consensus params returns 0",
			setupFn: func() servertypes.AppOptions {
				homeDir := createGenesisWithoutConsensusParams(t)
				opts := newMockAppOptions()
				opts.Set(flags.FlagHome, homeDir)
				return opts
			},
			expected: 0,
		},
		{
			name: "invalid genesis JSON returns 0",
			setupFn: func() servertypes.AppOptions {
				homeDir := createInvalidGenesis(t)
				opts := newMockAppOptions()
				opts.Set(flags.FlagHome, homeDir)
				return opts
			},
			expected: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(_ *testing.T) {
			appOpts := tc.setupFn()
			logger := log.NewNopLogger()

			result := GetBlockGasLimit(appOpts, logger)
			require.Equal(t, tc.expected, result, "GetBlockGasLimit returned unexpected value")
		})
	}
}

func TestGetMinGasPrices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setupFn  func() servertypes.AppOptions
		expected sdk.DecCoins
	}{
		{
			name: "valid single gas price",
			setupFn: func() servertypes.AppOptions {
				opts := newMockAppOptions()
				opts.Set(sdkserver.FlagMinGasPrices, "0.025uatom")
				return opts
			},
			expected: sdk.DecCoins{sdk.NewDecCoinFromDec("uatom", sdkmath.LegacyMustNewDecFromStr("0.025"))},
		},
		{
			name: "valid multiple gas prices",
			setupFn: func() servertypes.AppOptions {
				opts := newMockAppOptions()
				opts.Set(sdkserver.FlagMinGasPrices, "0.025uatom,0.001stake")
				return opts
			},
			expected: sdk.DecCoins{
				sdk.NewDecCoinFromDec("stake", sdkmath.LegacyMustNewDecFromStr("0.001")),
				sdk.NewDecCoinFromDec("uatom", sdkmath.LegacyMustNewDecFromStr("0.025")),
			},
		},
		{
			name: "empty gas prices returns empty DecCoins",
			setupFn: func() servertypes.AppOptions {
				opts := newMockAppOptions()
				opts.Set(sdkserver.FlagMinGasPrices, "")
				return opts
			},
			expected: nil,
		},
		{
			name: "missing gas prices flag returns empty DecCoins",
			setupFn: func() servertypes.AppOptions {
				opts := newMockAppOptions()
				return opts
			},
			expected: nil,
		},
		{
			name: "invalid gas price format returns empty DecCoins",
			setupFn: func() servertypes.AppOptions {
				opts := newMockAppOptions()
				opts.Set(sdkserver.FlagMinGasPrices, "invalid-format")
				return opts
			},
			expected: sdk.DecCoins{},
		},
		{
			name: "malformed coin denomination returns empty DecCoins",
			setupFn: func() servertypes.AppOptions {
				opts := newMockAppOptions()
				opts.Set(sdkserver.FlagMinGasPrices, "0.025")
				return opts
			},
			expected: sdk.DecCoins{},
		},
		{
			name: "zero amount gas price",
			setupFn: func() servertypes.AppOptions {
				opts := newMockAppOptions()
				opts.Set(sdkserver.FlagMinGasPrices, "0uatom")
				return opts
			},
			expected: sdk.DecCoins{},
		},
		{
			name: "large decimal precision gas price",
			setupFn: func() servertypes.AppOptions {
				opts := newMockAppOptions()
				opts.Set(sdkserver.FlagMinGasPrices, "0.000000000000000001uatom")
				return opts
			},
			expected: sdk.DecCoins{sdk.NewDecCoinFromDec("uatom", sdkmath.LegacyMustNewDecFromStr("0.000000000000000001"))},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(_ *testing.T) {
			appOpts := tc.setupFn()
			logger := log.NewNopLogger()

			result := GetMinGasPrices(appOpts, logger)
			require.Equal(t, tc.expected, result, "GetMinGasPrices returned unexpected value")
		})
	}
}

func createGenesisWithMaxGas(t *testing.T, maxGas int64) string {
	t.Helper()
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	genesis := map[string]interface{}{
		"app_name":       "evmd",
		"app_version":    "test",
		"chain_id":       "test-chain",
		"initial_height": 1,
		"genesis_time":   "2024-01-01T00:00:00Z",
		"app_hash":       nil,
		"app_state": map[string]interface{}{
			"auth": map[string]interface{}{
				"params": map[string]interface{}{
					"max_memo_characters":       "256",
					"tx_sig_limit":              "7",
					"tx_size_cost_per_byte":     "10",
					"sig_verify_cost_ed25519":   "590",
					"sig_verify_cost_secp256k1": "1000",
				},
				"accounts": []interface{}{},
			},
		},
		"consensus": map[string]interface{}{
			"params": map[string]interface{}{
				"block": map[string]interface{}{
					"max_bytes": "22020096",
					"max_gas":   fmt.Sprintf("%d", maxGas),
				},
				"evidence": map[string]interface{}{
					"max_age_num_blocks": "100000",
					"max_age_duration":   "172800000000000",
					"max_bytes":          "1048576",
				},
				"validator": map[string]interface{}{
					"pub_key_types": []string{"ed25519"},
				},
				"version": map[string]interface{}{
					"app": "0",
				},
			},
		},
	}

	genesisBytes, err := json.MarshalIndent(genesis, "", "  ")
	require.NoError(t, err)

	genesisPath := filepath.Join(configDir, "genesis.json")
	require.NoError(t, os.WriteFile(genesisPath, genesisBytes, 0o600))

	return tempDir
}

func createGenesisWithoutConsensusParams(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	genesis := map[string]interface{}{
		"app_name":       "evmd",
		"app_version":    "test",
		"chain_id":       "test-chain",
		"initial_height": 1,
		"genesis_time":   "2024-01-01T00:00:00Z",
		"app_hash":       nil,
		"app_state": map[string]interface{}{
			"auth": map[string]interface{}{
				"params":   map[string]interface{}{},
				"accounts": []interface{}{},
			},
		},
		"consensus": map[string]interface{}{
			"params": nil,
		},
	}

	genesisBytes, err := json.MarshalIndent(genesis, "", "  ")
	require.NoError(t, err)

	genesisPath := filepath.Join(configDir, "genesis.json")
	require.NoError(t, os.WriteFile(genesisPath, genesisBytes, 0o600))

	return tempDir
}

func createInvalidGenesis(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o755))

	invalidJSON := `{"invalid": json}`
	genesisPath := filepath.Join(configDir, "genesis.json")
	require.NoError(t, os.WriteFile(genesisPath, []byte(invalidJSON), 0o600))

	return tempDir
}
