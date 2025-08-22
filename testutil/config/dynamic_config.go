package config

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/spf13/viper"

	cosmosevmserverconfig "github.com/cosmos/evm/server/config"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	serverconfig "github.com/cosmos/cosmos-sdk/server/config"
)

// DynamicChainConfig allows creating arbitrary chain configurations for testing
// without hardcoded case switches.
type DynamicChainConfig struct {
	ChainID       string
	EVMChainID    uint64
	Denom         string
	ExtendedDenom string
	DisplayDenom  string
	Decimals      uint8
}

// TestConfigTemplate is the combined template for generating test configurations
const TestConfigTemplate = serverconfig.DefaultConfigTemplate + cosmosevmserverconfig.DefaultEVMConfigTemplate

// EVMAppConfig defines the combined configuration structure that matches
// what the template expects. This should match evmd/cmd/evmd/config/evmd_config.go
type EVMAppConfig struct {
	serverconfig.Config `mapstructure:",squash"`

	EVM     cosmosevmserverconfig.EVMConfig     `mapstructure:"evm"`
	JSONRPC cosmosevmserverconfig.JSONRPCConfig `mapstructure:"json-rpc"`
	TLS     cosmosevmserverconfig.TLSConfig     `mapstructure:"tls"`
	Chain   cosmosevmserverconfig.ChainConfig   `mapstructure:"chain"`
}

// GenerateConfigFromTemplate creates a TOML configuration string from the template
// using the provided parameters. This eliminates the need for hardcoded configurations.
func GenerateConfigFromTemplate(chainCfg DynamicChainConfig) (string, error) {
	// Create the base server config
	srvCfg := serverconfig.DefaultConfig()
	srvCfg.MinGasPrices = "0" + chainCfg.Denom

	// Create the EVM config
	evmCfg := cosmosevmserverconfig.DefaultEVMConfig()
	evmCfg.EVMChainID = chainCfg.EVMChainID

	// Create the chain config
	chainConfig := cosmosevmserverconfig.ChainConfig{
		Denom:         chainCfg.Denom,
		ExtendedDenom: chainCfg.ExtendedDenom,
		DisplayDenom:  chainCfg.DisplayDenom,
		Decimals:      chainCfg.Decimals,
	}

	// Combine into the structure the template expects
	appConfig := EVMAppConfig{
		Config:  *srvCfg,
		EVM:     *evmCfg,
		JSONRPC: *cosmosevmserverconfig.DefaultJSONRPCConfig(),
		TLS:     *cosmosevmserverconfig.DefaultTLSConfig(),
		Chain:   chainConfig,
	}

	// Parse and execute the template
	tmpl, err := template.New("config").Parse(TestConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse config template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, appConfig); err != nil {
		return "", fmt.Errorf("failed to execute config template: %w", err)
	}

	return buf.String(), nil
}

// ParseConfigFromTOML parses a TOML configuration string and returns the parsed config
func ParseConfigFromTOML(tomlContent string) (*cosmosevmserverconfig.Config, error) {
	v := viper.New()
	v.SetConfigType("toml")

	if err := v.ReadConfig(bytes.NewBufferString(tomlContent)); err != nil {
		return nil, fmt.Errorf("failed to read TOML config: %w", err)
	}

	config, err := cosmosevmserverconfig.GetConfig(v)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// CreateEvmCoinInfoFromDynamicConfig creates an EvmCoinInfo from dynamic parameters
// without hardcoded case switches.
func CreateEvmCoinInfoFromDynamicConfig(chainCfg DynamicChainConfig) evmtypes.EvmCoinInfo {
	return evmtypes.EvmCoinInfo{
		Denom:         chainCfg.Denom,
		ExtendedDenom: chainCfg.ExtendedDenom,
		DisplayDenom:  chainCfg.DisplayDenom,
		Decimals:      evmtypes.Decimals(chainCfg.Decimals),
	}
}

// GenerateAndParseConfig is a convenience function that generates a TOML config
// from parameters and returns both the TOML string and parsed config.
func GenerateAndParseConfig(chainCfg DynamicChainConfig) (string, *cosmosevmserverconfig.Config, error) {
	tomlContent, err := GenerateConfigFromTemplate(chainCfg)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate config: %w", err)
	}

	config, err := ParseConfigFromTOML(tomlContent)
	if err != nil {
		return tomlContent, nil, fmt.Errorf("failed to parse generated config: %w", err)
	}

	return tomlContent, config, nil
}

// Common test configurations that can be created dynamically
var (
	// DefaultTestChain provides a standard 18-decimal test configuration
	DefaultTestChain = DynamicChainConfig{
		ChainID:       "cosmos-1",
		EVMChainID:    9001,
		Denom:         "aatom",
		ExtendedDenom: "aatom",
		DisplayDenom:  "atom",
		Decimals:      18,
	}

	// SixDecimalsTestChain provides a 6-decimal test configuration
	SixDecimalsTestChain = DynamicChainConfig{
		ChainID:       "ossix-2",
		EVMChainID:    9002,
		Denom:         "utest",
		ExtendedDenom: "atest",
		DisplayDenom:  "test",
		Decimals:      6,
	}

	// TwelveDecimalsTestChain provides a 12-decimal test configuration
	TwelveDecimalsTestChain = DynamicChainConfig{
		ChainID:       "ostwelve-3",
		EVMChainID:    9003,
		Denom:         "ptest2",
		ExtendedDenom: "atest2",
		DisplayDenom:  "test2",
		Decimals:      12,
	}
)

// CreateCustomTestChain allows creating a test chain with custom parameters
// without needing to add cases to hardcoded switch statements.
func CreateCustomTestChain(chainID string, evmChainID uint64, denom, displayDenom string, decimals uint8) DynamicChainConfig {
	extendedDenom := denom
	// For non-18 decimals, create a different extended denom
	if decimals != 18 {
		extendedDenom = "a" + displayDenom
	}

	return DynamicChainConfig{
		ChainID:       chainID,
		EVMChainID:    evmChainID,
		Denom:         denom,
		ExtendedDenom: extendedDenom,
		DisplayDenom:  displayDenom,
		Decimals:      decimals,
	}
}
