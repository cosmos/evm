package config

import (
	"fmt"
	"strconv"

	"github.com/spf13/cast"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/client/flags"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	srvflags "github.com/cosmos/evm/server/flags"
)

// LoadChainConfig loads chain configuration from CLI flags and configuration files.
// It follows this priority order:
// 1. CLI flags (highest priority)
// 2. TOML configuration file
// 3. Default values (lowest priority)
func LoadChainConfig(appOpts servertypes.AppOptions) (ChainConfig, error) {
	// Load Chain ID
	chainID, err := loadChainID(appOpts)
	if err != nil {
		return ChainConfig{}, fmt.Errorf("failed to load chain ID: %w", err)
	}

	// Load EVM Chain ID with defaults
	evmChainID, err := loadEVMChainID(appOpts)
	if err != nil {
		return ChainConfig{}, fmt.Errorf("failed to load EVM chain ID: %w", err)
	}

	// Load Coin Info with defaults
	coinInfo, err := loadCoinInfoWithDefaults(appOpts)
	if err != nil {
		return ChainConfig{}, fmt.Errorf("failed to load coin info: %w", err)
	}

	// Build ChainConfig using existing structure
	chainConfig := ChainConfig{
		ChainInfo: ChainInfo{
			ChainID:    chainID,
			EVMChainID: evmChainID,
		},
		CoinInfo: coinInfo,
	}

	return chainConfig, nil
}

// loadChainID loads the Cosmos chain ID from appOpts, following this priority:
// 1. --chain-id flag
// 2. Home directory client config
// 3. Empty string (will need to be set elsewhere)
func loadChainID(appOpts servertypes.AppOptions) (string, error) {
	// Try to get from CLI flag first
	chainID := cast.ToString(appOpts.Get(flags.FlagChainID))
	if chainID != "" {
		return chainID, nil
	}

	// If not available, try to load from home directory
	homeDir := cast.ToString(appOpts.Get(flags.FlagHome))
	if homeDir != "" {
		chainID, err := GetChainIDFromHome(homeDir)
		if err == nil && chainID != "" {
			return chainID, nil
		}
		// Don't error if we can't read from home - just continue with empty
	}

	return "", nil
}

// loadEVMChainID loads the EVM chain ID from appOpts, following this priority:
// 1. --evm.evm-chain-id flag
// 2. TOML configuration [evm] evm-chain-id
// 3. Default value (9001) as fallback
func loadEVMChainID(appOpts servertypes.AppOptions) (uint64, error) {
	// Try EVM-specific flag first
	if evmFlag := appOpts.Get(srvflags.EVMChainID); evmFlag != nil {
		if evmFlagStr := cast.ToString(evmFlag); evmFlagStr != "" {
			evmChainID, err := strconv.ParseUint(evmFlagStr, 10, 64)
			if err == nil {
				return evmChainID, nil
			}
			// If we have a flag value but can't parse it, that's an error
			return 0, fmt.Errorf("failed to parse EVM chain ID from flag: %w", err)
		}
	}

	// Try TOML configuration
	if tomlValue := appOpts.Get("evm.evm-chain-id"); tomlValue != nil {
		if tomlStr := cast.ToString(tomlValue); tomlStr != "" {
			evmChainID, err := strconv.ParseUint(tomlStr, 10, 64)
			if err == nil {
				return evmChainID, nil
			}
		}
	}

	// Fall back to default value (matches local_node.sh CHAINID default)
	return 9001, nil
}

// loadCoinInfo loads coin configuration from appOpts.
// All coin configuration must be explicitly specified in TOML configuration [chain] section.
// Returns error if any required values are missing.
func loadCoinInfo(appOpts servertypes.AppOptions) (evmtypes.EvmCoinInfo, error) {
	coinInfo := evmtypes.EvmCoinInfo{}

	// All fields are required
	denom := cast.ToString(appOpts.Get("coin.denom"))
	if denom == "" {
		return coinInfo, fmt.Errorf("coin.denom must be specified in [coin] section of app.toml")
	}
	coinInfo.Denom = denom

	extendedDenom := cast.ToString(appOpts.Get("coin.extended-denom"))
	if extendedDenom == "" {
		return coinInfo, fmt.Errorf("coin.extended-denom must be specified in [coin] section of app.toml")
	}
	coinInfo.ExtendedDenom = extendedDenom

	displayDenom := cast.ToString(appOpts.Get("coin.display-denom"))
	if displayDenom == "" {
		return coinInfo, fmt.Errorf("coin.display-denom must be specified in [coin] section of app.toml")
	}
	coinInfo.DisplayDenom = displayDenom

	decimals := cast.ToUint64(appOpts.Get("coin.decimals"))
	if decimals == 0 {
		return coinInfo, fmt.Errorf("coin.decimals must be specified and non-zero in [coin] section of app.toml")
	}
	coinInfo.Decimals = evmtypes.Decimals(decimals)

	return coinInfo, nil
}

// loadCoinInfoWithDefaults loads coin configuration with fallback to default values
func loadCoinInfoWithDefaults(appOpts servertypes.AppOptions) (evmtypes.EvmCoinInfo, error) {
	// Try to load from configuration first
	coinInfo, err := loadCoinInfo(appOpts)
	if err == nil {
		return coinInfo, nil
	}

	// If loading from config fails, provide sensible defaults
	// These match the values from local_node.sh environment variables
	defaultDenom := "atest"
	defaultDisplayDenom := "test"
	defaultDecimals := evmtypes.Decimals(18)

	// Create extended denom (atto version for 18-decimal representation)
	extendedDenom := evmtypes.CreateDenomStr(defaultDecimals, defaultDisplayDenom)

	// If the base denom has different decimals, we need to create it properly
	denom := evmtypes.CreateDenomStr(defaultDecimals, defaultDisplayDenom)
	if defaultDecimals == evmtypes.EighteenDecimals {
		// For 18 decimals, base and extended are the same
		denom = defaultDenom
		extendedDenom = defaultDenom
	}

	coinInfo = evmtypes.EvmCoinInfo{
		Denom:         denom,
		ExtendedDenom: extendedDenom,
		DisplayDenom:  defaultDisplayDenom,
		Decimals:      defaultDecimals,
	}

	return coinInfo, nil
}
