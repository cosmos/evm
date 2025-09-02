# EpixChain

**A high-performance EVM-compatible blockchain built on Cosmos SDK with custom tokenomics and advanced DeFi features.**

EpixChain is a specialized blockchain that combines the power of the Cosmos SDK with full Ethereum Virtual Machine (EVM) compatibility, featuring a unique dynamic minting system and comprehensive DeFi infrastructure.

## 🌟 Key Features

- **🔥 Dynamic Token Emission**: Custom EpixMint module with 25% annual reduction reaching 42B EPIX max supply over 20 years
- **⚡ EVM Compatibility**: Full Ethereum compatibility with native Cosmos SDK integration
- **🌉 IBC Integration**: Seamless cross-chain transfers and interoperability
- **💰 Native DeFi**: Built-in staking, governance, and wrapped token functionality
- **🔧 Precompiled Contracts**: Optimized smart contracts for core blockchain functions
- **🛡️ Enterprise Ready**: Comprehensive testing, verification, and monitoring tools

## 📊 Network Information

| Network | Chain ID | RPC Endpoint | REST API |
|---------|----------|--------------|----------|
| **Mainnet** | 1916 | `https://rpc.epixchain.com` | `https://api.epixchain.com` |
| **Testnet** | 1917 | `http://localhost:8545` | `http://localhost:1317` |

## 💎 EPIX Token

- **Base Denomination**: `aepix` (1 EPIX = 10^18 aepix)
- **Display Denomination**: `epix`
- **Decimals**: 18
- **Maximum Supply**: 42,000,000,000 EPIX (42 billion)
- **Initial Annual Emission**: 10.527 billion EPIX (Year 1)
- **Reduction Rate**: 25% annually

## 📋 Deployed Contracts

EpixChain comes with a comprehensive set of predeployed contracts for enhanced functionality:

### 🏭 Infrastructure Contracts

| Contract | Address | Description |
|----------|---------|-------------|
| **Create2 Factory** | `0x4e59b44847b379578588920ca78fbf26c0b4956c` | Deterministic contract deployment |
| **Multicall3** | `0xcA11bde05977b3631167028862bE2a173976CA11` | Batch multiple calls in single transaction |
| **Permit2** | `0x000000000022D473030F116dDEE9F6B43aC78BA3` | Universal token approval system |
| **Safe Singleton Factory** | `0x914d7Fec6aaC8cd542e72Bca78B30650d45643d7` | Safe wallet deployment factory |
| **EIP-2935 History Storage** | `0x0aae40965e6800cd9b1f4b05ff21581047e3f91e` | Block hash history storage |

### 🪙 Native Token Contracts

| Contract | Address | Description |
|----------|---------|-------------|
| **Native EPIX Token** | `0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE` | Native EPIX token precompile |
| **WEPIX (Wrapped EPIX)** | `0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2` | Wrapped EPIX for DeFi compatibility |

### ⚙️ Cosmos Module Precompiles

EpixChain provides native access to Cosmos SDK modules through EVM precompiles:

| Module | Function | Status |
|--------|----------|--------|
| **Distribution** | Staking rewards & delegation management | ✅ Active |
| **Staking** | Validator operations & delegations | ✅ Active |
| **Bank** | Token transfers & balances | ✅ Active |
| **Governance** | Proposal voting & authorization | ✅ Active |
| **IBC Transfer** | Cross-chain asset transfers | ✅ Active |
| **EVM** | Smart contract execution | ✅ Active (10 precompiles) |

## 🚀 Quick Start

### Prerequisites

- Go 1.21+
- Node.js 16+
- Git

### Installation

```bash
# Clone the repository
git clone https://github.com/EpixZone/EpixChain.git
cd EpixChain

# Build the binary
make install

# Verify installation
epixd version
```

### Setup Local Network

```bash
# Setup and start local testnet with contract verification
./scripts/setup_epix_chain.sh --verify-contracts --network mainnet

# Start the node
epixd start --home ~/.epixd \
  --pruning nothing \
  --json-rpc.api eth,txpool,personal,net,debug,web3 \
  --chain-id 1916
```

### Connect with MetaMask

Add EpixChain to EVM Wallet (MetaMask):

- **Network Name**: EpixChain Testnet
- **RPC URL**: `http://rpc.epix.zone`
- **Chain ID**: `1916`
- **Currency Symbol**: `EPIX`
- **Block Explorer**: `http://scan.epix.zone`

## 🏗️ EpixMint Module

EpixChain features a custom minting module with dynamic token emission:

### Tokenomics Overview

- **Initial Emission**: 10.527 billion EPIX in Year 1
- **Annual Reduction**: 25% per year
- **Target Timeline**: 20 years to reach maximum supply
- **Maximum Supply**: 42 billion EPIX total
- **Block Time**: 6 seconds (configurable via governance)

### Emission Schedule

| Year | Annual Emission | Cumulative Supply |
|------|----------------|-------------------|
| 1 | 10.527B EPIX | 10.527B EPIX |
| 2 | 7.895B EPIX | 18.422B EPIX |
| 5 | 3.331B EPIX | 32.156B EPIX |
| 10 | 563M EPIX | 40.891B EPIX |
| 20 | 16M EPIX | ~42B EPIX |

### Key Features

- **Smooth Exponential Decay**: Continuous per-block reduction instead of annual steps
- **Block-time Awareness**: Automatic adjustment for consensus changes
- **Maximum Supply Protection**: Multiple safeguards prevent exceeding 42B EPIX
- **Governance Control**: All parameters updatable via on-chain governance

### 🔄 Automatic Block Time Adjustment

EpixMint automatically adjusts token emission when block times change - **no manual intervention required!**

**How it works:**
- Monitors actual block production times (averages last 100 blocks)
- Automatically recalculates tokens per block to maintain annual emission targets
- Seamlessly handles governance-driven block time changes

**Example: 6 seconds → 2 seconds**
```
Before: 6s blocks = 5.26M blocks/year → 2,002 EPIX per block
After:  2s blocks = 15.8M blocks/year → 667 EPIX per block
Result: Same 10.527B EPIX annual emission maintained
```

This ensures consistent tokenomics regardless of consensus parameter changes.

## 🏛️ Governance

EpixChain uses on-chain governance to allow stakeholders to propose and vote on parameter changes. This includes modifying EpixMint parameters like block time, emission rates, and distribution ratios.

**📖 [Complete Governance Guide](docs/governance-guide.md)** - Learn how to create and submit governance proposals


## 🔧 Contract Verification

Verify all deployed contracts are active on your network:

```bash
# Verify contracts on testnet
./scripts/setup_epix_chain.sh --verify-contracts --network testnet

# Verify contracts on mainnet
./scripts/setup_epix_chain.sh --verify-contracts --network mainnet
```

The verification script checks:
- ✅ Infrastructure contract bytecode and functionality
- ✅ Native token precompile activation
- ✅ Cosmos module REST API accessibility
- ✅ EVM precompile configuration
- ✅ WEPIX deployment and token pair registration

## 🛡️ Security & Audit

For detailed audit findings and security analysis, see the [Sherlock Audit Report](./docs/audits/sherlock_2025_07_28_final.pdf).

## 🧪 Development & Testing

### Unit Testing

```bash
make test-unit
```

### Coverage Testing

```bash
make test-unit-cover
```

### Solidity Contract Testing

```bash
make test-solidity
```

### Fuzz Testing

```bash
make test-fuzz
```

### Benchmark Testing

```bash
make benchmark
```

## 🔗 EVM Features

### JSON-RPC Compatibility

Full Ethereum JSON-RPC API support:
- `eth_*` - Ethereum standard methods
- `net_*` - Network information
- `web3_*` - Web3 utilities
- `txpool_*` - Transaction pool management
- `debug_*` - Debug and tracing
- `personal_*` - Account management

### Wallet Integration

Compatible with all major Ethereum wallets:
- MetaMask
- WalletConnect
- Rabby
- Trust Wallet
- Coinbase Wallet

### Developer Tools

Works with standard Ethereum development tools:
- Hardhat
- Foundry
- Remix
- Truffle
- Web3.js
- Ethers.js
- Viem

## 🏛️ Governance

EpixChain uses on-chain governance for protocol upgrades:

- **Proposal Submission**: Stake-weighted proposal creation
- **Voting Period**: Community voting on proposals
- **Parameter Updates**: Modify chain parameters via governance
- **Upgrade Coordination**: Seamless protocol upgrades

## 🌐 IBC & Interoperability

### Cross-Chain Features

- **IBC Transfers**: Native cross-chain asset transfers
- **Token Bridging**: Seamless asset movement between chains
- **Interchain Accounts**: Execute transactions on remote chains
- **Packet Forwarding**: Multi-hop IBC routing

### Supported Networks

EpixChain connects to the broader Cosmos ecosystem including:
- Cosmos Hub
- Osmosis
- Juno
- Stargaze
- And 50+ other IBC-enabled chains

## 📚 Documentation

- **Validator Onboarding**: [./docs/validator-onboarding.md](./docs/validator-onboarding.md) - Complete guide for running validator nodes
- **Setup Script**: [./scripts/setup_epix_chain.sh](./scripts/setup_epix_chain.sh)
- **EpixMint Module**: [./x/epixmint/README.md](./x/epixmint/README.md)
- **API Reference**: Available at `http://localhost:1317/swagger/` when running locally
- **JSON-RPC Docs**: Standard Ethereum JSON-RPC documentation applies

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines.

## 📄 License

EpixChain is open-source under the Apache 2.0 license. See [LICENSE](./LICENSE) for details.

## 🔗 Links

- **Website**: [https://epix.zone](https://epix.zone)
- **Documentation**: [https://docs.epix.zone](https://docs.epix.zone)
- **Explorer (Staking, Governance, L1 functions)**: [https://explorer.epix.zone](https://explorer.epix.zone)
- **Explorer (EVM Scan)**: [https://scan.epix.zone](https://explorer.epix.zone)
- **GitHub**: [https://github.com/EpixZone/EpixChain](https://github.com/EpixZone/EpixChain)

---

**Built with ❤️ by the EpixChain team**
