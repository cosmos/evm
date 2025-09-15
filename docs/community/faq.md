# Frequently Asked Questions (FAQ)

Find answers to the most common questions about EpixChain. If you don't find what you're looking for, join our [Discord community](https://discord.gg/epix) for help.

## General Questions

### What is EpixChain?

EpixChain is the world's first blockchain designed to power a completely decentralized internet. Built on the Cosmos SDK with full EVM compatibility, it enables websites to be hosted by everyone and controlled by no one, making the web as unstoppable as Bitcoin itself.

### How is EpixChain different from Ethereum?

| Feature | EpixChain | Ethereum |
|---------|-----------|----------|
| **Block Time** | 6 seconds | 12 seconds |
| **Finality** | Instant | 12+ minutes |
| **Fees** | Ultra-low | High (variable) |
| **Consensus** | Tendermint PoS | PoS (post-merge) |
| **Interoperability** | Native IBC | Bridges only |
| **Supply** | 42B fixed | No cap |
| **Purpose** | Decentralized web | General smart contracts |

### What makes EpixChain special?

1. **Built for the Decentralized Web**: Specifically designed to power EpixNet
2. **True Interoperability**: Native IBC connectivity to 50+ chains
3. **Fair Launch**: No investors or team allocations
4. **Low Fees**: Optimized for everyday use and microtransactions
5. **Fast Finality**: 6-second blocks with instant finality
6. **Community Governance**: Fully decentralized decision-making

## Tokenomics

### What is the EPIX token?

EPIX is the native token of EpixChain with the following specifications:
- **Symbol**: EPIX
- **Decimals**: 18
- **Max Supply**: 42 billion EPIX
- **Genesis Supply**: 23.7 million EPIX (airdropped)
- **Emission**: Dynamic with 25% annual reduction

### How does the emission schedule work?

EpixChain uses a custom EpixMint module with:
- **Year 1**: 10.527 billion EPIX minted
- **Annual Reduction**: 25% each year
- **Timeline**: 20 years to reach maximum supply
- **Protection**: Hard cap prevents exceeding 42B EPIX

### Was there a fair launch?

Yes! EpixChain had a completely fair launch:
- **0% Investor Allocation**: No VC or private sales
- **0% Team Allocation**: No pre-allocated team tokens
- **100% Community**: All tokens through airdrop and mining
- **Transparent**: Open-source and auditable

### How can I get EPIX tokens?

1. **Airdrop**: Claim if eligible from snapshot
2. **Staking**: Earn rewards by delegating to validators
3. **Exchanges**: Purchase from supported exchanges (coming soon)
4. **Cross-Chain**: Bridge from other networks
5. **Governance**: Participate in community proposals

## Technical Questions

### Is EpixChain EVM compatible?

Yes, EpixChain provides full Ethereum Virtual Machine compatibility:
- Deploy existing Ethereum contracts without modification
- Use familiar tools like MetaMask, Hardhat, and Remix
- Support for all Ethereum JSON-RPC methods
- Compatible with Web3.js, Ethers.js, and other libraries

### What consensus mechanism does EpixChain use?

EpixChain uses **Tendermint BFT consensus**:
- **Byzantine Fault Tolerant**: Up to 1/3 malicious validators
- **Instant Finality**: No confirmation delays
- **Energy Efficient**: Proof of Stake mechanism
- **Fast Blocks**: 6-second block times

### How does cross-chain communication work?

EpixChain uses **IBC (Inter-Blockchain Communication)**:
- **Native Protocol**: Built into Cosmos SDK
- **Trustless**: Cryptographic proofs verify transfers
- **Bidirectional**: Send and receive from other chains
- **Extensive**: Connect to 50+ IBC-enabled chains

### What are precompiled contracts?

Precompiled contracts provide native blockchain functionality through EVM:
- **Bank Operations**: Native token transfers
- **Staking**: Validator delegation through smart contracts
- **Governance**: Vote on proposals from dApps
- **IBC**: Cross-chain transfers via EVM
- **10+ Precompiles**: Enhanced functionality

## Usage Questions

### How do I connect my wallet?

#### MetaMask (Recommended)
1. Add EpixChain network with Chain ID 1916
2. RPC URL: `https://rpc.epixchain.com`
3. Currency Symbol: EPIX
4. Block Explorer: `https://scan.epix.zone`

#### Keplr (Cosmos Ecosystem)
1. Visit a compatible dApp
2. Approve EpixChain addition
3. Switch to EpixChain network

### What are the network fees like?

EpixChain fees are designed to be ultra-low:
- **Simple Transfer**: ~$0.001
- **Smart Contract**: ~$0.01-0.10
- **Complex Operations**: Still under $1
- **Gas Token**: Paid in EPIX

### How do I stake EPIX?

1. **Choose Validator**: Research commission and performance
2. **Delegate Tokens**: Use wallet or staking interface
3. **Earn Rewards**: ~8-15% APR (varies)
4. **Compound**: Automatically or manually restake
5. **Unbond**: 21-day unbonding period

### Can I participate in governance?

Yes! EPIX holders can participate in governance:
- **Voting Power**: 1 staked EPIX = 1 vote
- **Proposal Types**: Parameter changes, upgrades, spending
- **Voting Options**: Yes, No, Abstain, No with Veto
- **Requirements**: Must stake EPIX to vote

## Development Questions

### Can I deploy my Ethereum dApp on EpixChain?

Yes! EpixChain is fully EVM compatible:
- **No Code Changes**: Deploy existing contracts as-is
- **Same Tools**: Use Hardhat, Truffle, Remix
- **Same Libraries**: Web3.js, Ethers.js work perfectly
- **Lower Costs**: Significantly cheaper than Ethereum

### What development tools are supported?

All standard Ethereum tools work:
- **Frameworks**: Hardhat, Foundry, Truffle
- **IDEs**: Remix, VS Code with Solidity
- **Libraries**: Web3.js, Ethers.js, CosmJS
- **Testing**: Same testing frameworks as Ethereum

### Are there grants for developers?

Yes! EpixChain supports developers through:
- **Community Pool**: Governance-controlled funding
- **Developer Grants**: For innovative projects
- **Hackathons**: Regular events with prizes
- **Technical Support**: Active developer community

### How do I get testnet tokens?

For development purposes:
1. **Faucet**: Request tokens from testnet faucet
2. **Local Node**: Run local development chain
3. **Community**: Ask in Discord #developers channel

## Validator Questions

### How do I become a validator?

1. **Technical Setup**: Run validator node with required specs
2. **Stake Requirement**: Self-delegate minimum EPIX
3. **Commission**: Set commission rate for delegators
4. **Community**: Build reputation and attract delegators

### What are the hardware requirements?

**Minimum Requirements**:
- 4 CPU cores
- 8GB RAM
- 200GB SSD storage
- 100 Mbps internet

**Recommended**:
- 8+ CPU cores
- 32GB+ RAM
- 1TB+ NVMe SSD
- 1 Gbps internet

### What are the risks of validating?

**Slashing Conditions**:
- **Double Sign**: 5% of stake slashed
- **Downtime**: 1% of stake slashed (after 95% uptime threshold)
- **Tombstoning**: Permanent jail for severe violations

**Mitigation**:
- Use reliable infrastructure
- Implement monitoring and alerting
- Have backup systems ready
- Keep validator software updated

## Security Questions

### Is EpixChain secure?

Yes, EpixChain inherits security from proven technologies:
- **Cosmos SDK**: Battle-tested framework
- **Tendermint**: Years of production use
- **Audited Code**: Professional security audits
- **Open Source**: Transparent and reviewable

### How do I keep my tokens safe?

**Wallet Security**:
- Use hardware wallets for large amounts
- Never share seed phrases
- Verify all transactions before signing
- Use official wallet applications only

**Best Practices**:
- Enable 2FA where possible
- Keep software updated
- Be cautious of phishing attempts
- Use strong, unique passwords

### What if I lose my private keys?

**Prevention**:
- Backup seed phrases securely
- Store in multiple locations
- Test recovery process
- Consider multi-signature setups

**If Lost**:
- Tokens cannot be recovered without keys
- No central authority can help
- This is why backups are critical

## Community Questions

### How can I get involved?

**Community Participation**:
- Join [Discord](https://discord.gg/epix) for discussions
- Follow [@zone_epix](https://x.com/zone_epix) on Twitter
- Participate in governance voting
- Contribute to GitHub repositories

**Technical Contribution**:
- Submit bug reports and feature requests
- Contribute code improvements
- Write documentation
- Help other community members

### Where can I get support?

**Community Support**:
- **Discord**: Real-time help from community
- **GitHub**: Technical issues and bug reports
- **Documentation**: Comprehensive guides
- **Twitter**: Updates and announcements

**Emergency Support**:
- Critical security issues: security@epix.zone
- Validator issues: validators channel in Discord
- Technical problems: developers channel in Discord

### How is EpixChain governed?

**Decentralized Governance**:
- **Proposals**: Anyone can submit (with deposit)
- **Voting**: All EPIX holders can participate
- **Implementation**: Automatic execution of passed proposals
- **Transparency**: All votes and proposals are public

**Governance Process**:
1. Proposal submission (1,000 EPIX deposit)
2. 7-day voting period
3. Quorum and threshold requirements
4. Automatic implementation if passed

---

*Still have questions? Join our [Discord community](https://discord.gg/epix) where our team and community members are happy to help!*
