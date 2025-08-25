package config

import (
	evmtypes "github.com/cosmos/evm/x/vm/types"
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
