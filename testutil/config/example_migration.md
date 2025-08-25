# Chain Configuration Migration Guide

This guide explains the current approach to EVM chain configuration and shows how to use the dynamic configuration utilities for testing.

## Current State

The codebase currently uses hardcoded switch statements for chain configuration:

```go
// Current approach in testutil/constants/constants.go
func GetExampleChainCoinInfo(chainID ChainID) evmtypes.EvmCoinInfo {
    switch chainID {
    case ExampleChainID:
        return evmtypes.EvmCoinInfo{
            Denom:         ExampleAttoDenom,
            ExtendedDenom: ExampleAttoDenom,
            DisplayDenom:  ExampleDisplayDenom,
            Decimals:      evmtypes.EighteenDecimals,
        }
    case SixDecimalsChainID:
        return evmtypes.EvmCoinInfo{
            Denom:         "utest",
            ExtendedDenom: "atest",
            DisplayDenom:  "test",
            Decimals:      evmtypes.SixDecimals,
        }
    // ... more hardcoded cases
    }
}
```

## Dynamic Configuration for Tests

For more flexible testing, use the dynamic configuration system in `testutil/config`:

```go
import "github.com/cosmos/evm/testutil/config"

// Create custom chain configuration
chainCfg := config.DynamicChainConfig{
    ChainID:       "test-chain-123",
    EVMChainID:    9001,
    Denom:         "utoken",
    ExtendedDenom: "atoken", 
    DisplayDenom:  "token",
    Decimals:      18,
}

// Create EvmCoinInfo directly
evmCoinInfo := config.CreateEvmCoinInfoFromDynamicConfig(chainCfg)
```

### Predefined Test Configurations

```go
// Available predefined configurations
config.DefaultTestChain        // 18 decimals, aatom/atom
config.SixDecimalsTestChain    // 6 decimals, utest/atest/test  
config.TwelveDecimalsTestChain // 12 decimals, ptest2/atest2/test2
```

## Test Examples

### Basic Usage
```go
func TestBasicConfig(t *testing.T) {
    // Use predefined configuration
    coinInfo := config.CreateEvmCoinInfoFromDynamicConfig(config.DefaultTestChain)
    
    assert.Equal(t, "aatom", coinInfo.Denom)
    assert.Equal(t, "atom", coinInfo.DisplayDenom)
    assert.Equal(t, evmtypes.EighteenDecimals, coinInfo.Decimals)
}
```

### Custom Configuration
```go
func TestCustomConfig(t *testing.T) {
    chainCfg := config.DynamicChainConfig{
        ChainID:       "custom-test",
        EVMChainID:    12345,
        Denom:         "ucustom",
        ExtendedDenom: "acustom",
        DisplayDenom:  "custom",
        Decimals:      8,
    }
    
    coinInfo := config.CreateEvmCoinInfoFromDynamicConfig(chainCfg)
    
    assert.Equal(t, "ucustom", coinInfo.Denom)
    assert.Equal(t, "acustom", coinInfo.ExtendedDenom)
    assert.Equal(t, "custom", coinInfo.DisplayDenom)
    assert.Equal(t, uint8(8), uint8(coinInfo.Decimals))
}
```

## Migration Path

When migrating tests:

1. **Simple cases**: Use predefined configurations
   ```go
   // Instead of: testconstants.GetExampleChainCoinInfo(chainID)
   // Use: config.CreateEvmCoinInfoFromDynamicConfig(config.DefaultTestChain)
   ```

2. **Custom cases**: Define your own `DynamicChainConfig`
   ```go
   chainCfg := config.DynamicChainConfig{...}
   coinInfo := config.CreateEvmCoinInfoFromDynamicConfig(chainCfg)
   ```

3. **Dynamic test cases**: Use `CreateCustomTestChain` for easy parameterization
   ```go
   chainCfg := config.CreateCustomTestChain("test-1", 9001, "utoken", "token", 18)
   coinInfo := config.CreateEvmCoinInfoFromDynamicConfig(chainCfg)
   ```

The hardcoded switch statements remain for backward compatibility but can be gradually replaced with the dynamic approach where needed.
