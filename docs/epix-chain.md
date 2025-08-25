# Epix Chain Configuration

This document describes the Epix blockchain configuration and tokenomics implementation.

## Overview

Epix is a custom blockchain built on the Cosmos EVM framework with unique tokenomics designed for sustainable growth and community governance.

## Chain Information

### Mainnet
- **Chain ID**: `epix_1916-1`
- **EVM Chain ID**: `1916`
- **RPC**: `https://rpc.epix.zone`
- **API**: `https://api.epix.zone`
- **Symbol**: `EPIX`
- **Decimals**: `18`

### Testnet
- **Chain ID**: `epix_1917-1`
- **EVM Chain ID**: `1917`
- **RPC**: `https://rpc.testnet.epix.zone`
- **API**: `https://api.testnet.epix.zone`
- **Symbol**: `EPIX`
- **Decimals**: `18`

## Tokenomics

### Genesis Supply
- **Total Genesis Supply**: 23,689,538 EPIX
- **Airdrop Allocation**: 11,844,769 EPIX (50%)
- **Community Pool**: 11,844,769 EPIX (50%)

### Minting Schedule
- **Initial Minting Rate**: 10.527 billion EPIX per year
- **Reduction Period**: 1 year
- **Reduction Rate**: 25% annually (75% retention rate)
- **Timeline**: 20 years
- **Maximum Supply**: 42 billion EPIX

### Minting Formula
```
Year N Minting Rate = Initial Rate × (0.75)^(N-1)
```

**Example Schedule:**
- Year 1: 10.527B EPIX
- Year 2: 7.895B EPIX (10.527B × 0.75)
- Year 3: 5.921B EPIX (7.895B × 0.75)
- Year 4: 4.441B EPIX (5.921B × 0.75)
- ...and so on

## Technical Implementation

### Custom Mint Function
The Epix chain uses a custom mint function (`EpixMintFn`) that implements the unique tokenomics:

1. **Time-based Minting**: Minting rate decreases every year based on elapsed time
2. **Deterministic Schedule**: Predictable token emission over 20 years
3. **Max Supply Enforcement**: Built-in protection against exceeding 42B EPIX
4. **Governance Compatible**: Can be modified through governance proposals

### Key Components

#### 1. Tokenomics Structure (`EpixTokenomics`)
- Defines all tokenomics parameters
- Calculates current minting rate based on elapsed time
- Provides block-level provision calculations

#### 2. Custom Mint Function (`EpixMintFn`)
- Replaces the standard Cosmos SDK mint function
- Implements Epix-specific minting logic
- Emits appropriate events for monitoring

#### 3. Genesis Configuration
- Sets up initial token distribution
- Configures mint module parameters
- Establishes denomination metadata

## Building and Running

### Prerequisites
- Go 1.23.8 or later
- Git

### Build the Binary
```bash
# Using make (recommended)
make build

# Or manually
cd evmd
go build -o epixd ./cmd/evmd/
```

### Initialize a Node

#### Using the Initialization Script
```bash
# Initialize testnet node
./scripts/init-epix-chain.sh --network testnet

# Initialize mainnet node
./scripts/init-epix-chain.sh --network mainnet

# Custom initialization
./scripts/init-epix-chain.sh --network testnet --moniker "my-node" --home "/custom/path"
```

#### Manual Initialization
```bash
# Initialize node
epixd init <moniker> --chain-id epix_1917-1

# Add validator key
epixd keys add validator

# Add genesis account
epixd genesis add-genesis-account $(epixd keys show validator -a) 1000000000000000000000000aepix

# Create genesis transaction
epixd genesis gentx validator 1000000000000000000000aepix --chain-id epix_1917-1

# Collect genesis transactions
epixd genesis collect-gentxs

# Start the node
epixd start
```

## Configuration Files

### Key Files
- `evmd/epix_mint.go`: Custom mint function implementation
- `evmd/epix_config.go`: Chain configuration helpers
- `evmd/genesis.go`: Genesis state configuration
- `evmd/cmd/evmd/config/constants.go`: Chain constants

### Genesis Customization
The genesis state can be customized for different deployment scenarios:

```go
// Get Epix-specific genesis
genesis, err := NewEpixAppGenesisForChain(1917) // testnet
if err != nil {
    panic(err)
}

// Customize as needed
// genesis[banktypes.ModuleName] = customBankGenesis
```

## Monitoring and Queries

### Check Minting Status
```bash
# Query current inflation
epixd query mint inflation

# Query annual provisions
epixd query mint annual-provisions

# Query mint parameters
epixd query mint params
```

### Check Supply
```bash
# Query total supply
epixd query bank total

# Query supply of specific denom
epixd query bank total --denom aepix
```

## Governance

The Epix chain supports governance proposals to modify:
- Mint parameters (through governance)
- Chain upgrades
- Parameter changes
- Community pool spending

## Security Considerations

1. **Deterministic Minting**: The minting schedule is deterministic and cannot be manipulated
2. **Max Supply Protection**: Built-in safeguards prevent exceeding the maximum supply
3. **Time-based Logic**: Minting is based on block time, making it predictable
4. **Governance Oversight**: Critical parameters can be modified through governance

## Development

### Adding New Features
1. Modify the appropriate configuration files
2. Update the custom mint function if needed
3. Test thoroughly on testnet
4. Submit governance proposal for mainnet changes

### Testing
```bash
# Run tests
make test

# Build and test locally
make build
./build/epixd --help
```

## Support

For technical support and questions:
- GitHub Issues: [EpixZone/evm](https://github.com/EpixZone/evm)
- Documentation: This file and inline code comments
- Community: Epix Discord/Telegram channels
