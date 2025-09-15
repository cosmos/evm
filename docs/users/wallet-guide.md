# Wallet Guide for EpixChain

This comprehensive guide will help you set up, configure, and use various wallets with EpixChain. Whether you're new to crypto or an experienced user, this guide covers everything you need to know.

## Supported Wallets

EpixChain supports both EVM-compatible wallets and Cosmos ecosystem wallets, giving you flexibility in how you interact with the network.

### EVM Wallets (Recommended for Beginners)

- **MetaMask** - Most popular browser extension wallet
- **Trust Wallet** - Mobile wallet with built-in dApp browser
- **Coinbase Wallet** - User-friendly with institutional backing
- **Brave Wallet** - Built into Brave browser
- **WalletConnect** - Connect to dApps with mobile wallets

### Cosmos Wallets (Advanced Features)

- **Keplr** - Leading Cosmos ecosystem wallet
- **Cosmostation** - Mobile and web wallet for Cosmos chains
- **Leap Wallet** - Modern Cosmos wallet with advanced features

## MetaMask Setup (Recommended)

### Installation

1. **Download MetaMask**:
   - Visit [metamask.io](https://metamask.io)
   - Install browser extension or mobile app
   - Create new wallet or import existing one

2. **Secure Your Wallet**:
   - Write down your seed phrase securely
   - Never share your seed phrase with anyone
   - Use a strong password
   - Enable biometric authentication (mobile)

### Adding EpixChain Network

1. **Open MetaMask** and click the network dropdown

2. **Add Network** and select "Add a network manually"

3. **Enter Network Details**:

#### Mainnet Configuration
```
Network Name: EpixChain Mainnet
New RPC URL: https://rpc.epixchain.com
Chain ID: 1916
Currency Symbol: EPIX
Block Explorer URL: https://scan.epix.zone
```

#### Testnet Configuration
```
Network Name: EpixChain Testnet
New RPC URL: http://localhost:8545
Chain ID: 1917
Currency Symbol: EPIX
Block Explorer URL: http://localhost:8080
```

4. **Save** and switch to EpixChain network

### Importing EPIX Token

If EPIX doesn't appear automatically:

1. **Click "Import tokens"** in MetaMask
2. **Enter token details**:
   - Token Contract Address: `0x...` (native token)
   - Token Symbol: `EPIX`
   - Token Decimal: `18`
3. **Add Token**

## Keplr Setup (Advanced)

### Installation and Setup

1. **Install Keplr**:
   - Visit [keplr.app](https://keplr.app)
   - Install browser extension
   - Create or import wallet

2. **Add EpixChain**:
   ```javascript
   // EpixChain will be added automatically when visiting compatible dApps
   // Or manually add through Keplr's chain registry
   ```

3. **Configure Chain**:
   - Chain ID: `epix_1916-1` (mainnet) or `epix_1917-1` (testnet)
   - RPC: `https://rpc.epixchain.com`
   - REST: `https://api.epixchain.com`

## Getting EPIX Tokens

### Airdrop Claim (If Eligible)

1. **Check Eligibility**:
   - Visit [epix.zone](https://epix.zone)
   - Connect your wallet
   - Check if you're eligible for the airdrop

2. **Claim Process**:
   - Click "Claim Airdrop"
   - Sign the transaction
   - Wait for confirmation
   - Tokens will appear in your wallet

### Purchasing EPIX

#### From Exchanges (Coming Soon)
- Centralized exchanges will list EPIX
- Decentralized exchanges on EpixChain
- Cross-chain DEXs with IBC support

#### Cross-Chain Bridges
```bash
# Example: Bridge from Ethereum
1. Use supported bridge (coming soon)
2. Connect source wallet (Ethereum)
3. Connect destination wallet (EpixChain)
4. Bridge tokens with small fee
```

### Earning EPIX

#### Staking Rewards
- Delegate EPIX to validators
- Earn ~8-15% APR (varies by network conditions)
- Compound rewards automatically

#### Governance Participation
- Vote on proposals
- Some proposals may include reward distributions
- Active participation benefits the ecosystem

#### Liquidity Provision
- Provide liquidity to DEXs
- Earn trading fees and liquidity rewards
- Higher risk but potentially higher returns

## Basic Wallet Operations

### Sending EPIX

#### Using MetaMask
1. **Open MetaMask** and ensure you're on EpixChain network
2. **Click "Send"**
3. **Enter recipient address** (starts with `0x` for EVM or `epix` for Cosmos)
4. **Enter amount** in EPIX
5. **Review gas fee** (should be very low)
6. **Confirm transaction**

#### Using Keplr
1. **Open Keplr** and select EpixChain
2. **Click "Send"**
3. **Enter recipient address** (Cosmos format: `epix1...`)
4. **Enter amount** and select EPIX
5. **Set gas fee** (usually auto-calculated)
6. **Confirm transaction**

### Receiving EPIX

1. **Copy your address**:
   - MetaMask: Click account name to copy address
   - Keplr: Click "Copy Address" button

2. **Share address** with sender

3. **Wait for confirmation**:
   - EpixChain has 6-second block times
   - Transactions are final after 1 block

### Checking Balance and History

#### MetaMask
- Balance shown on main screen
- Click "Activity" tab for transaction history
- Use block explorer for detailed information

#### Keplr
- Balance displayed prominently
- Transaction history in "Activity" section
- Detailed view available through explorer links

## Advanced Features

### Cross-Chain Transfers (IBC)

#### Using Keplr
1. **Navigate to IBC transfer** in supported dApps
2. **Select source and destination chains**
3. **Enter amount** to transfer
4. **Confirm transaction**
5. **Wait for relayer** to complete transfer (1-5 minutes)

#### Supported Chains
- Cosmos Hub
- Osmosis
- Juno
- Stargaze
- 50+ other IBC-enabled chains

### Staking Operations

#### Delegate to Validator
1. **Choose validator** from list
2. **Check commission rate** and performance
3. **Enter delegation amount**
4. **Confirm transaction**
5. **Start earning rewards** immediately

#### Claim Rewards
1. **View accumulated rewards**
2. **Click "Claim Rewards"**
3. **Choose to restake** or withdraw
4. **Confirm transaction**

#### Undelegate (Unbond)
1. **Select validator** to undelegate from
2. **Enter amount** to undelegate
3. **Confirm transaction**
4. **Wait 21 days** for unbonding period
5. **Tokens return** to available balance

### Governance Participation

#### Viewing Proposals
1. **Navigate to governance** section
2. **Browse active proposals**
3. **Read proposal details** carefully
4. **Check voting deadline**

#### Voting Process
1. **Select proposal** to vote on
2. **Choose vote option**:
   - Yes: Support the proposal
   - No: Oppose the proposal
   - Abstain: Participate without taking a side
   - No with Veto: Strong opposition (burns deposit)
3. **Confirm vote transaction**

## Security Best Practices

### Wallet Security

1. **Seed Phrase Protection**:
   - Write down on paper, never digital
   - Store in multiple secure locations
   - Never share with anyone
   - Test recovery process

2. **Password Security**:
   - Use strong, unique passwords
   - Enable biometric authentication
   - Don't save passwords in browsers
   - Use password managers

3. **Device Security**:
   - Keep devices updated
   - Use antivirus software
   - Avoid public WiFi for transactions
   - Lock devices when not in use

### Transaction Security

1. **Verify Addresses**:
   - Double-check recipient addresses
   - Use address book for frequent recipients
   - Be aware of address poisoning attacks

2. **Gas Fee Awareness**:
   - Understand gas fees before confirming
   - Don't overpay for simple transactions
   - Be cautious of extremely high gas estimates

3. **Smart Contract Interactions**:
   - Only interact with verified contracts
   - Understand what you're signing
   - Revoke unnecessary approvals
   - Use reputable dApps only

### Common Scams to Avoid

1. **Fake Websites**: Always verify URLs
2. **Phishing Emails**: Never click suspicious links
3. **Social Engineering**: No legitimate support asks for seed phrases
4. **Fake Airdrops**: Be cautious of unexpected token claims
5. **Impersonation**: Verify official social media accounts

## Troubleshooting

### Common Issues

#### "Transaction Failed"
- **Check gas fees**: Ensure sufficient EPIX for gas
- **Network congestion**: Try again later or increase gas
- **Wrong network**: Verify you're on EpixChain

#### "Insufficient Funds"
- **Check balance**: Ensure you have enough EPIX
- **Account for gas**: Reserve EPIX for transaction fees
- **Unstaked tokens**: Check if tokens are staked

#### "Network Error"
- **Check internet**: Ensure stable connection
- **Try different RPC**: Use alternative RPC endpoint
- **Clear cache**: Reset wallet cache if needed

### Getting Help

1. **Documentation**: Check these docs first
2. **Community Discord**: [discord.gg/epix](https://discord.gg/epix)
3. **GitHub Issues**: [github.com/EpixZone/EpixChain](https://github.com/EpixZone/EpixChain)
4. **Block Explorer**: [scan.epix.zone](https://scan.epix.zone)

### Emergency Recovery

If you lose access to your wallet:

1. **Use seed phrase** to recover in new wallet
2. **Import private keys** if available
3. **Contact support** if funds are stuck
4. **Never share recovery information** with anyone

---

*Remember: Your wallet security is your responsibility. Always verify information and never share sensitive details with anyone.*
